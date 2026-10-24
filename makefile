version := $(shell git describe --tags --always)
time := $(shell date)

svartvit:*.go go.mod
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit .

linux:
	GOOS=linux GOARCH=arm GOARM=6 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-linux-arm-$(version) .

macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static" -X "main.BuildVersion=$(version)" -X "main.BuildTime=$(time)"' -o svartvit-macos-amd-64-$(version) .

# Give up when the connection to the Pi stalls instead of waiting forever.
ssh_options := -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=3

# The binary can't be overwritten while it runs, so upload it next to the old one and rename it.
deploy:linux
	scp $(ssh_options) svartvit-linux-arm-$(version) pi@svartvit:app/svartvit.new
	ssh $(ssh_options) pi@svartvit 'mv app/svartvit.new app/svartvit && sudo systemctl restart svartvit'
