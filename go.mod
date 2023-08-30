module github.com/skrassiev/meerkat

go 1.18

require (
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/stretchr/testify v1.7.0
)

replace github.com/fsnotify/fsnotify v1.6.0 => github.com/skrassiev/fsnotify v1.6.0-closewrite-b1

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.0 // indirect
)
