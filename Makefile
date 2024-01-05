all:
	go build cmd/main.go

armh:
	GOARM=6 GOOS=linux GOARCH=arm go build -o meerkat cmd/main.go

test:
	go test -v ./... -args --public-ip=$$(curl api.ipify.org)
