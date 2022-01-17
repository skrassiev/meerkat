all:
	go build cmd/standalone/main.go

pi2:
	GOOS=linux GOARCH=arm go build cmd/standalone/main.go

pi3:
	GOOS=linux GOARCH=arm go build cmd/fs/main.go

test:
	go test -v ./... -args --public-ip=$$(curl api.ipify.org)
