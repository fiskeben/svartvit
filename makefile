version := $(shell git describe --tags --always)
time := $(shell date)

svartvit:*.go go.mod
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit .

linux:
	GOOS=linux GOARCH=arm GOARM=6 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-linux-arm-$(version) .

macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-macos-amd-64-$(version) .

deploy:linux
	scp svartvit-linux-arm-$(version) pi@bilderamme:app/svartvit
