# Testing

## Test successful punlic IP

```bash
TEST_PUBLIC_IP=$(curl ifconfig.io) go test -v ./...
```


## Block remote service on linux

To test failing remote service, such as api.ipify.org, block all IPs of that FQDN with:

```bash
sudo iptables -t filter -A OUTPUT -p tcp -d 104.21.192.0/24 -j DROP
go test -v
sudo iptables -t filter -F OUTPUT
go test -v
```

