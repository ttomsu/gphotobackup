module github.com/ttomsu/gphotobackup

go 1.23.0

toolchain go1.23.6

require (
	github.com/gphotosuploader/googlemirror v0.5.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	go.uber.org/zap v1.27.0
	golang.org/x/oauth2 v0.30.0
)

replace github.com/gphotosuploader/googlemirror v0.5.0 => github.com/ttomsu/googlemirror v0.6.0

require (
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/sagikazarmark/locafero v0.10.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/api v0.244.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
