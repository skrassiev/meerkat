all:
	go build cmd/standalone/main.go

rpi:
	GOOS=linux GOARCH=arm go build cmd/standalone/main.go


