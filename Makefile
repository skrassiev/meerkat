all:
	go build cmd/standalone/main.go

rpi:
	GOOS=linux GOARCH=arm go build cmd/standalone/main.go

test:
	go test -v ./... -args --public-ip=$$(curl ifconfig.io)
