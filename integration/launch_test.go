package integration

import (
	"context"
	"math"
	"testing"
	"time"

	krpcgo "github.com/atburke/krpc-go"
	"github.com/atburke/krpc-go/krpc"
	"github.com/atburke/krpc-go/spacecenter"
	"github.com/atburke/krpc-go/types"
	"github.com/stretchr/testify/require"
)

// TestLaunch starts from the space center, loads the Kerbal, X, and launches
// it into orbit. The procedure for launching the vessel into orbit is adapted
// from https://krpc.github.io/krpc/tutorials/launch-into-orbit.html. This
// function is tested with the Kerbal X starting on the KSC launchpad.
func TestLaunch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := krpcgo.NewKRPCClient(krpcgo.KRPCClientConfig{})
	require.NoError(t, client.Connect(ctx))
	t.Logf("Connected to %s:%s", client.Host, client.RPCPort)

	krpcService := krpc.New(client)
	// TODO: SetPaused causes problems with current mod version if called while at space center :(
	// require.NoError(t, krpcService.SetPaused(false))
	// t.Cleanup(func() {
	// 	require.NoError(t, krpcService.SetPaused(true))
	// })

	// Set stuff up
	sc := spacecenter.New(client)
	t.Log("Loading Space Center")
	require.NoError(t, sc.LoadSpaceCenter())
	t.Log("Requesting list of launchable vessels")
	vv, err := sc.LaunchableVessels("VAB")
	require.NoError(t, err)
	require.Contains(t, vv, "Kerbal X", "Current game doesn't have Kerbal X avaiable")

	// TODO: would be nice if kRPC had a way to get the whole roster
	k, err := sc.GetKerbal("Tester Kerman")
	require.NoError(t, err)
	if k.ID_internal() == 0 { //null :)
		t.Log("Creating Tester Kerman")
		require.NoError(t, sc.CreateKerbal("Tester Kerman", "Pilot", true))
	}
	_, err = sc.GetKerbal("Tester Kerman")
	require.NoError(t, err)

	t.Log("Loading Kerbal X on the Launch Pad")
	require.NoError(t, sc.LaunchVessel("VAB", "Kerbal X", "LaunchPad", true, []string{"Tester Kerman"}, ""))

	t.Log("Switching back to Space Center leaving vessel on pad")
	require.NoError(t, sc.LoadSpaceCenter())

	k, err = sc.GetKerbal("Tester2 Kerman")
	require.NoError(t, err)
	if k.ID_internal() == 0 { //null :)
		t.Log("Creating Tester2 Kerman")
		require.NoError(t, sc.CreateKerbal("Tester2 Kerman", "Pilot", true))
	}
	_, err = sc.GetKerbal("Tester2 Kerman")
	require.NoError(t, err)

	t.Log("Loading Kerbal X on the Launch Pad again, expecting an error")
	require.Error(t, sc.LaunchVessel("VAB", "Kerbal X", "LaunchPad", false, []string{"Tester2 Kerman"}, ""),
		"Expected an error due to launch pad not being clear")

	t.Log("Loading Kerbal X on the Launch Pad again again")
	require.NoError(t, sc.LaunchVessel("VAB", "Kerbal X", "LaunchPad", true, []string{"Tester2 Kerman"}, ""))

	gamescene, err := krpcService.CurrentGameScene()
	require.NoError(t, err)
	require.Equal(t, krpc.GameScene_Flight, gamescene, "Expected to be Flight scene")

	vessel, err := sc.ActiveVessel()
	require.NoError(t, err)
	vesselName, err := vessel.Name()
	require.NoError(t, err)
	t.Logf("Current vessel name: %s", vesselName)

	t.Log("Setting up for launch")
	rf, err := vessel.SurfaceReferenceFrame()
	require.NoError(t, err)
	flight, err := vessel.Flight(rf)
	require.NoError(t, err)
	orbit, err := vessel.Orbit()
	require.NoError(t, err)

	altitudeStream, err := flight.MeanAltitudeStream()
	require.NoError(t, err)
	apoapsisStream, err := orbit.ApoapsisAltitudeStream()
	require.NoError(t, err)
	qStream, err := flight.DynamicPressureStream()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, altitudeStream.Close())
		require.NoError(t, apoapsisStream.Close())
		require.NoError(t, qStream.Close())
	})

	control, err := vessel.Control()
	require.NoError(t, err)
	require.NoError(t, control.SetSAS(false))
	require.NoError(t, control.SetRCS(false))
	require.NoError(t, control.SetThrottle(1.0))

	autopilot, err := vessel.AutoPilot()
	require.NoError(t, err)

	t.Log("Launch!")
	_, err = control.ActivateNextStage()
	require.NoError(t, err)
	require.NoError(t, autopilot.Engage())
	require.NoError(t, autopilot.TargetPitchAndHeading(90.0, 90))

	// Autostaging
	go func() {
		t.Log("Autostaging gofunc starting")
		defer t.Log("Autostaging finished")
		stage, err := control.CurrentStage()
		require.NoError(t, err)

		for {
			t.Logf("current stage is %v\n", stage)
			resources, err := vessel.ResourcesInDecoupleStage(stage-1, false)
			require.NoError(t, err)
			amountStream, err := resources.AmountStream("LiquidFuel")
			require.NoError(t, err)

		readAmount:
			for {
				select {
				case amount := <-amountStream.C:
					if amount < 0.1 {
						_, err = control.ActivateNextStage()
						require.NoError(t, amountStream.Close())
						require.NoError(t, err)
						stage--
						if stage == 0 {
							return
						}
						break readAmount
					}
				case <-ctx.Done():
					return
				}
			}

		}
	}()

	turnStartAltitude := 250.0
	turnEndAltitude := 45000.0
	targetAltitude := 150000.0

	turnAngle := 0.0
	var apoapsis float64

	limitingThrottle := false

	t.Logf("Waiting for appoapsis >= %0.2f", 0.9*targetAltitude)
	for apoapsis < 0.9*targetAltitude {
		select {
		// Manage heading
		case altitude := <-altitudeStream.C:
			if altitude < turnStartAltitude || altitude > turnEndAltitude {
				continue
			}
			frac := (altitude - turnStartAltitude) / (turnEndAltitude - turnStartAltitude)
			newTurnAngle := frac * 90
			if math.Abs(newTurnAngle-turnAngle) > 0.5 {
				turnAngle = newTurnAngle
				require.NoError(t, autopilot.TargetPitchAndHeading(float32(90-turnAngle), 90))
			}
		case apoapsis = <-apoapsisStream.C:

			// Lazy Q limiting
		case q := <-qStream.C:
			if q >= 20000 && !limitingThrottle {
				limitingThrottle = true
				require.NoError(t, control.SetThrottle(0.5))
			} else if q < 20000 && limitingThrottle {
				limitingThrottle = false
				require.NoError(t, control.SetThrottle(1.0))
			}
		case <-ctx.Done():
			return
		}
	}
	t.Logf("Appoapsis hit target: %0.2f", apoapsis)

	t.Log("Fine tunning appoapsis approach")
	require.NoError(t, control.SetThrottle(0.25))
	for apoapsis < targetAltitude {
		select {
		case apoapsis = <-apoapsisStream.C:
		case <-ctx.Done():
			return
		}
	}
	require.NoError(t, control.SetThrottle(0))

	t.Log("Coasting to edge of the atmosphere")
	for apoapsis < 70500 {
		select {
		case apoapsis = <-apoapsisStream.C:
		case <-ctx.Done():
			return
		}
	}

	t.Log("Calculating circularization maneuver")
	body, err := orbit.Body()
	require.NoError(t, err)
	mu, err := body.GravitationalParameter()
	require.NoError(t, err)
	r, err := orbit.Apoapsis()
	require.NoError(t, err)
	a1, err := orbit.SemiMajorAxis()
	require.NoError(t, err)
	a2 := r
	v1 := math.Sqrt(float64(mu) * ((2 / r) - (1 / a1)))
	v2 := math.Sqrt(float64(mu) * ((2 / r) - (1 / a2)))
	deltaV := v2 - v1
	ut, err := sc.UT()
	require.NoError(t, err)
	timeToApoapsis, err := orbit.TimeToApoapsis()
	require.NoError(t, err)
	node, err := control.AddNode(ut+timeToApoapsis, float32(deltaV), 0, 0)
	require.NoError(t, err)

	t.Log("Calculating burn time")
	f, err := vessel.AvailableThrust()
	require.NoError(t, err)
	rawISP, err := vessel.SpecificImpulse()
	require.NoError(t, err)
	isp := float64(rawISP * 9.82)
	m0, err := vessel.Mass()
	require.NoError(t, err)
	m1 := float64(m0) / math.Exp(deltaV/isp)
	flowRate := float64(f) / isp
	burnTime := (float64(m0) - m1) / flowRate

	t.Log("Orienting ship")
	require.NoError(t, control.SetRCS(true))
	nodeRF, err := node.ReferenceFrame()
	require.NoError(t, err)
	require.NoError(t, autopilot.SetReferenceFrame(nodeRF))
	require.NoError(t, autopilot.SetTargetDirection(types.NewVector3D(0, 1, 0).Tuple()))
	require.NoError(t, autopilot.Wait())

	t.Log("Waiting until burn")
	ut, err = sc.UT()
	require.NoError(t, err)
	timeToApoapsis, err = orbit.TimeToApoapsis()
	require.NoError(t, err)
	burnUT := ut + timeToApoapsis - (burnTime / 2)
	leadTime := float64(5)
	require.NoError(t, sc.WarpTo(burnUT-leadTime, 10, 1))

	t.Log("Executing burn")
	timeToApoapsisStream, err := orbit.TimeToApoapsisStream()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, timeToApoapsisStream.Close())
	})
	for timeToApoapsis-(burnTime/2) > 0 {
		select {
		case timeToApoapsis = <-timeToApoapsisStream.C:
		case <-ctx.Done():
			return
		}
	}

	require.NoError(t, control.SetThrottle(1.0))
	time.Sleep(time.Duration(math.Round((burnTime - 0.1) * float64(time.Second))))
	require.NoError(t, control.SetThrottle(0.05))

	remainingBurnStream, err := node.RemainingDeltaVStream()
	require.NoError(t, err)
	remainingBurn := <-remainingBurnStream.C
	for remainingBurn > 5 {
		select {
		case remainingBurn = <-remainingBurnStream.C:
		case <-ctx.Done():
			return
		}
	}

	require.NoError(t, control.SetThrottle(0))
	require.NoError(t, node.Remove())

}
