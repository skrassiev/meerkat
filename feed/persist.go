package feed

import (
	"errors"
	"net"
	"os"
	"path"
	"path/filepath"

	"log"
)

const (
	serviceName       = "meerkat"
	ipAddressFilename = "ip.address"
)

var (
	datadirDaemon   = "/var/cache/" + serviceName
	datadirUsermode = os.Getenv("HOME") + "/.cache/" + serviceName
)

func getStorageDir() string {
	for _, v := range []string{datadirDaemon, datadirUsermode} {
		fs, err := os.Stat(v)

		if err == nil {
			if fs.IsDir() {
				return v
			}
		} else if errors.Is(err, os.ErrNotExist) {
			// try creating path
			if err = os.Mkdir(v, 0766); err == nil {
				return v
			}
		}
	}

	return ""
}

type IPv4 string

func (ip *IPv4) read() string {
	dir := getStorageDir()
	f, err := os.Open(filepath.Join(dir, ipAddressFilename))
	if err != nil {
		return ""
	}
	defer f.Close()

	var ipAddrBytes [4*3 + 3]byte
	if n, err := f.Read(ipAddrBytes[:]); err == nil && n >= (4+3) {
		if parsedIP := net.ParseIP(string(ipAddrBytes[:n])); parsedIP != nil {
			*ip = IPv4(parsedIP.String())
			return parsedIP.String()
		}
	}
	return ""
}

// Write save IP address in the persistent storage. Given the logic of operations, we can ignore errors as persisting IP is only an optimization
func (ip IPv4) write() {
	if getStorageDir() == "" {
		return
	}
	if f, err := os.Create(path.Join(getStorageDir(), ipAddressFilename)); err == nil {
		if _, err = f.Write([]byte(ip)); err != nil {
			log.Println("WARN: failed to persist IPv4", err)
		}
		f.Close()
	} else {
		log.Println("WARN: failed to create persist file IPv4", err)
	}
}
