package sensor

import (
	"flag"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	assertPublicIP = flag.String("public-ip", "", "assert against public IP")
	noConn         = false
)

func TestMain(m *testing.M) {
	flag.Parse()
	conn, err := net.DialTimeout("tcp", "ifconfig.io:80", time.Second*3)
	if err != nil {
		noConn = true
	} else {
		conn.Close()
	}
	log.Println("err", err)
	os.Exit(m.Run())
}

func TestPublicIP_OK(t *testing.T) {
	if !noConn {
		if assertPublicIP == nil || len(*assertPublicIP) == 0 {
			*assertPublicIP = os.Getenv("TEST_PUBLIC_IP")
		}

		assert.Equal(t, *assertPublicIP, getPublicIP())
		assert.Equal(t, "", getPublicIP())
	}
}

func TestPublicIP_ConnectFailure(t *testing.T) {
	if noConn {
		assert.Equal(t, "", getPublicIP())
	}
}

func TestPublicIP_404(t *testing.T) {
	if !noConn {
		origURL := publicIPResolverURL
		defer func() { publicIPResolverURL = origURL }()
		publicIPResolverURL = "http://ifconfig.io/safdaf/sgsdg/sgd"
		assert.Equal(t, "", getPublicIP())
	}
}

func TestPublicIP_403(t *testing.T) {
	origURL := publicIPResolverURL
	defer func() { publicIPResolverURL = origURL }()
	publicIPResolverURL = "https://www.google.com/search?q=vim"
	assert.Equal(t, "", getPublicIP())
}

func TestPublicIP_BigBody(t *testing.T) {
	origURL := publicIPResolverURL
	defer func() { publicIPResolverURL = origURL }()
	publicIPResolverURL = "https://en.wikipedia.org/wiki/Vim_(text_editor)"
	assert.Equal(t, "", getPublicIP())
}
