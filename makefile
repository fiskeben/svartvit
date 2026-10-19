version := $(shell git describe --tags --always)
time := $(shell date)

svartvit:*.go go.mod
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit .

linux:
	GOOS=linux GOARCH=arm GOARM=6 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-linux-arm-$(version) .

macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-macos-amd-64-$(version) .

# The binary can't be overwritten while it runs, so upload it next to the old one and rename it.
deploy:linux
	scp svartvit-linux-arm-$(version) pi@svartvit:app/svartvit.new
	ssh pi@svartvit 'mv app/svartvit.new app/svartvit && sudo systemctl restart svartvit'
