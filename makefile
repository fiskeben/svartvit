version := $(shell git describe --tags --always)
time := $(shell date)

photoframe:*.go go.mod
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o photoframe .

linux:
	GOOS=linux GOARCH=arm GOARM=6 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o photoframe-linux-amd-64-$(version) .

macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o photoframe-macos-amd-64-$(version) .
