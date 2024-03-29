package main

import (
	"flag"

	"github.com/skrassiev/meerkat/bootstrap"
)

var (
	fServiceModeCommands    = flag.Bool("mode-commands", false, "respond to commands (temp, pic)")
	fServiceModePeriodic    = flag.Bool("mode-periodic", false, "periodic background tasks (IP-change)")
	fServiceModeFSMon       = flag.Bool("mode-fsmon", false, "monitor file system for images")
	fServiceModeHealthcheck = flag.Bool("mode-healthcheck", false, "ping-pong")
	fServiceModeTempMonitor = flag.Bool("mode-tempmon", false, "monitor and report temp changes more than 0.5")
)

func main() {
	flag.Parse()
	var runmode byte
	if *fServiceModeCommands {
		runmode |= bootstrap.ServiceModeCommands
	}
	if *fServiceModePeriodic {
		runmode |= bootstrap.ServiceModePeriodic
	}
	if *fServiceModeFSMon {
		runmode |= bootstrap.ServiceModeFSMoinitor
	}
	if *fServiceModeHealthcheck {
		runmode |= bootstrap.ServiceModeHealthcheck
	}
	if *fServiceModeTempMonitor {
		runmode |= bootstrap.ServiceModeTempMonitor
	}

	_, _ = bootstrap.Main("process", runmode)
}
