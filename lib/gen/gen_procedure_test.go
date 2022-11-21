package gen

const testProcedure = `
package gentest

import (
	api "github.com/atburke/krpc-go/api"
	encode "github.com/atburke/krpc-go/encode"
	client "github.com/atburke/krpc-go/lib/client"
	tracerr "github.com/ztrue/tracerr"
)

// MyProcedure will test procedure generation.
//
// Allowed game scenes: FLIGHT.
func (s *MyService) MyProcedure(param1 uint64, param2 string) (bool, error) {
	var err error
	var argBytes []byte
	var value bool
	request := &api.ProcedureCall{
		Procedure: "MyProcedure",
		Service: "MyService",
	}
	argBytes, err = encode.Marshal(param1)
	if err != nil {
		return value, tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x0),
		Value: argBytes,
	})
	argBytes, err = encode.Marshal(param2)
	if err != nil {
		return value, tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x1),
		Value: argBytes,
	})
	result, err := s.Client.Call(request, true)
	if err != nil {
		return value, tracerr.Wrap(err)
	}
	err = encode.Unmarshal(result, &value)
	if err != nil {
		return value, tracerr.Wrap(err)
	}
	return value, nil
}

// StreamMyProcedure will test procedure generation.
//
// Allowed game scenes: FLIGHT.
func (s *MyService) StreamMyProcedure(param1 uint64, param2 string) (*client.Stream[bool], error) {
	var err error
	var argBytes []byte
	request := &api.ProcedureCall{
		Procedure: "MyProcedure",
		Service: "MyService",
	}
	argBytes, err = encode.Marshal(param1)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x0),
		Value: argBytes,
	})
	argBytes, err = encode.Marshal(param2)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x1),
		Value: argBytes,
	})
	krpc := NewKRPC(s.Client)
	id, err := krpc.AddStream(request)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	rawStream := s.Client.GetStream(id)
	stream := client.MapStream(rawStream, func(b []byte)bool {
		var value bool
		encode.Unmarshal(b, &value)
		return value
	})
	return stream, nil
}
`

const testClassSetter = `
package gentest

import (
	api "github.com/atburke/krpc-go/api"
	encode "github.com/atburke/krpc-go/encode"
	tracerr "github.com/ztrue/tracerr"
)

// SetMyProperty will test class setter generation.
//
// Allowed game scenes: any.
func (s *MyClass) SetMyProperty(param1 api.Tuple2[string, uint64]) error {
	var err error
	var argBytes []byte
	request := &api.ProcedureCall{
		Procedure: "MyClass_set_MyProperty",
		Service: "MyService",
	}
	argBytes, err = encode.Marshal(s)
	if err != nil {
		return tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x0),
		Value: argBytes,
	})
	argBytes, err = encode.Marshal(param1)
	if err != nil {
		return tracerr.Wrap(err)
	}
	request.Arguments = append(request.Arguments, &api.Argument{
		Position: uint32(0x1),
		Value: argBytes,
	})
	result, err := s.Client.Call(request, false)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}
`
