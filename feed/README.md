# Testing

## Block remote service on linux

```bash
sudo iptables -t filter -A OUTPUT -p tcp -d 104.21.192.0/24 -j DROP
go test -v
sudo iptables -t filter -F OUTPUT
go test -v
```

