package feed

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
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
	apiURL, _ := url.Parse(publicIPResolverURL)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", apiURL.Hostname(), func() uint16 {
		switch apiURL.Scheme {
		case "http":
			return 80
		case "https":
			return 443
		default:
		}
		return 0
	}()), time.Second*3)
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

		assert.Equal(t, *assertPublicIP, PublicIP())
		assert.Equal(t, "", PublicIP())
	}
}

func TestPublicIP_ConnectFailure(t *testing.T) {
	if noConn {
		assert.Equal(t, "", PublicIP())
	}
}

func TestPublicIP_404(t *testing.T) {
	if !noConn {
		origURL := publicIPResolverURL
		defer func() { publicIPResolverURL = origURL }()
		publicIPResolverURL = publicIPResolverURLs[0] + "/safdaf/sgsdg/sgd"
		assert.Equal(t, "", PublicIP())
	}
}

func TestPublicIP_403(t *testing.T) {
	origURL := publicIPResolverURL
	defer func() { publicIPResolverURL = origURL }()
	publicIPResolverURL = "https:// www.google.com/search?q=vim"
	assert.Equal(t, "", PublicIP())
}

func TestPublicIP_BigBody(t *testing.T) {
	origURL := publicIPResolverURL
	defer func() { publicIPResolverURL = origURL }()
	publicIPResolverURL = "https:// en.wikipedia.org/wiki/Vim_(text_editor)"
	assert.Equal(t, "", PublicIP())
}
