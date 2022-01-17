module github.com/skrassiev/meerkat

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.5.1
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/stretchr/testify v1.7.0
	github.com/takama/daemon v1.0.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/fsnotify/fsnotify v1.5.1 => github.com/skrassiev/fsnotify v1.5.1-closewrite
