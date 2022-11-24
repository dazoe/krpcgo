.PHONY: gen fmt test gen-clean

gen:
	go generate ./...

gen-clean:
	rm ./*/*.gen.go

fmt:
	gofmt -w .

test:
	go test ./...