package feed

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	publicIP             net.IP
	publicIPResolverURLs = []string{"http://ifconfig.io", "https://api.ipify.org"}
	publicIPResolverURL  = publicIPResolverURLs[1]
)

func onError(msg string, arg interface{}) string {
	log.Println(msg, arg)
	return ""
}

//PublicIP returns current public IP of the system as a string. Returns emptry string if not changed or could not be determined
func PublicIP() (ip_address string) {
	req, err := http.NewRequest(http.MethodGet, publicIPResolverURL, nil)
	if err != nil {
		return onError("error getting public IP", err)
	}

	// ifconfig.io does not like programmatic access
	req.Header.Add("User-Agent", "curl/7.74.0")

	c := &http.Client{Timeout: time.Second * 30}
	resp, err := c.Do(req)

	if err != nil {
		return onError("error getting public IP", err)
	}

	if resp.StatusCode == 200 {

		if resp.ContentLength > int64(len("255.255.255.255\r")) {
			return onError("error getting public IP: long response", resp.ContentLength)
		}

		defer resp.Body.Close()
		body := make([]byte, 26)
		rbytes, err := resp.Body.Read(body)
		if err == nil || err == io.EOF {
			if rbytes >= len("1.1.1.1\r") && rbytes <= len("255.255.255.255\r") {
				newIP := net.ParseIP(strings.TrimSpace(string(body[0:rbytes])))
				if newIP != nil {
					if !newIP.Equal(publicIP) {
						publicIP = newIP
						return publicIP.String()
					}
					return ""
				}
			}
			return onError("error parsing public IP:", string(body))

		}
		return onError("error getting public IP:", err)

	}
	return onError("error code getting public IP", fmt.Sprintf("%d %s", resp.StatusCode, resp.Status))
}
