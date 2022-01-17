package main

import (
	"github.com/skrassiev/meerkat/bootstrap"
)

func main() {
	_, _ = bootstrap.Main("process", bootstrap.ServiceModeFSMoinitor|bootstrap.ServiceModeHealthcheck)
}
