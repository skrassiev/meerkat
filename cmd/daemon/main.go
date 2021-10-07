package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/skrassiev/gsnowmelt_bot/sensor"
	"github.com/takama/daemon"
)

const (

	// name of the service
	name        = "tempsensor"
	description = "Telegram Temperature Sensor Bot"
)

var systemDConfig = `[Unit]
Description={{.Description}}
After={{.Dependencies}}

[Service]
EnvironmentFile=-/etc/default/{{.Name}}
PIDFile=/run/{{.Name}}.pid
ExecStartPre=/bin/rm -f /run/{{.Name}}.pid
ExecStart={{.Path}} {{.Args}}
Restart=on-failure

[Install]
WantedBy=multi-user.target
`

//    dependencies that are NOT required by the service, but might be used
var dependencies = []string{"network.target"}

var stdlog, errlog *log.Logger

// Service has embedded daemon
type Service struct {
	daemon.Daemon
}

func usage() string {
	return fmt.Sprintf("Usage: %s install | remove | start | stop | status", name)
}

// Manage by daemon commands or run the daemon
func (service *Service) Manage() (string, error) {

	// if received any kind of command, do it
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":
			service.SetTemplate(systemDConfig)
			return service.Install()
		case "remove":
			return service.Remove()
		case "start":
			return service.Start()
		case "stop":
			return service.Stop()
		case "status":
			return service.Status()
		default:
			return usage(), nil
		}
	}

	// Do something, call your goroutines, etc

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// never happen, but need to complete code
	return sensor.ServeBotAPI(interrupt, "daemon")
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

func main() {
	srv, err := daemon.New(name, description, daemon.SystemDaemon, dependencies...)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}
	service := &Service{srv}
	status, err := service.Manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
}
