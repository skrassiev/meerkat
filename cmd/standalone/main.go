package main

import (
	"github.com/skrassiev/gsnowmelt_bot/bootstrap"
)

func main() {
	_, _ = bootstrap.Main("process", bootstrap.ServiceModeCommands|bootstrap.ServiceModePeriodic)
}
