<p align="center">
    <a href="https://pkg.go.dev/github.com/netologist/confucius?tab=doc"><img src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white" alt="godoc" title="godoc"/></a>
    <a href="https://travis-ci.org/hasanozgan/confucius"><img src="https://travis-ci.org/hasanozgan/confucius.svg?branch=master" alt="build status" title="build status"/></a>
    <a href="https://github.com/netologist/confucius/releases"><img src="https://img.shields.io/github/v/tag/hasanozgan/confucius" alt="semver tag" title="semver tag"/></a>
    <a href="https://goreportcard.com/report/github.com/netologist/confucius"><img src="https://goreportcard.com/badge/github.com/netologist/confucius" alt="go report card" title="go report card"/></a>
    <a href="https://github.com/netologist/confucius/blob/master/LICENSE"><img src="https://img.shields.io/github/license/hasanozgan/confucius" alt="license" title="license"/></a>
</p>

# confucius 222

confucius is forked from [kkyr/fig](https://github.com/kkyr/fig) project. fig is a tiny library for loading an application's config file and its environment into a Go struct. Individual fields can have default values defined or be marked as required.

I added extra features in project and send a PR. But they were not accepted. That reason I was forked that project.

## Why confucius?

- Define your **configuration**, **validations** and **defaults** in a single location
- Optionally **load from the environment** as well
- Optionally **profiles** as well
- You can use go:embed file system. You can find example usage in `examples/embed` folder
- Set environment variable in config file with default value
- Only **4** external dependencies
- Full support for`time.Time` & `time.Duration`
- Tiny API
- Decoders for `.yaml`, `.json` and `.toml` files
- Set String and Reader options for reference config. You can find example usage in `examples/reader` folder
- Added logger support

## Getting Started

`$ go get -d github.com/netologist/confucius`

Define your config file:

```yaml
# config.yaml

build: "2020-01-09T12:30:00Z"

server:
    ports:
      - "${SERVER_PORT:8080}"
    cleanup: 1h

logger:
    level: "warn"
    trace: true
```

Define your struct along with _validations_ or _defaults_:

```go
package main

import (
  "fmt"

  "github.com/netologist/confucius"
)

type Config struct {
  Build  time.Time `conf:"build" validate:"required"`
  Server struct {
    Host    string        `conf:"host" default:"127.0.0.1"`
    Ports   []int         `conf:"ports" default:"[80,443]"`
    Cleanup time.Duration `conf:"cleanup" default:"30m"`
  }
  Logger struct {
    Level string `conf:"level" default:"info"`
    Trace bool   `conf:"trace"`
  }
}

func main() {
  var cfg Config
  err := confucius.Load(&cfg)
  // handle your err
  
  fmt.Printf("%+v\n", cfg)
  // Output: {Build:2019-12-25 00:00:00 +0000 UTC Server:{Host:127.0.0.1 Ports:[8080] Cleanup:1h0m0s} Logger:{Level:warn Trace:true}}
}
```

If a field is not set and is marked as *required* then an error is returned. If a *default* value is defined instead then that value is used to populate the field.

Fig searches for a file named `config.yaml` in the directory it is run from. Change the lookup behaviour by passing additional parameters to `Load()`:

```go
confucius.Load(&cfg,
  confucius.File("settings.json"),
  confucius.Dirs(".", "/etc/myapp", "/home/user/myapp"),
) // searches for ./settings.json, /etc/myapp/settings.json, /home/user/myapp/settings.json

```

### Profiles

You can use `profiles` for other environments.

```go
confucius.Load(&cfg,
  confucius.File("settings.json"),
  confucius.Profiles("test", "integration")
  confucius.ProfileLayout("config-test.yaml") // DEFAULT: config.test.yaml
) // searches settings-test.json, settings-integration.json

```

### String and Reader

You can use `string or reader` for configuration

```go
confucius.Load(&cfg, 
  confucius.String(`{"application": {"port": 9000}}`, confucius.DecoderJSON),
)
```

```go
configReader := strings.NewReader(`{"application": {"port": 9000}}`)
confucius.Load(&cfg,
  confucius.Reader(configReader, confucius.DecoderJSON),
)
```

### go:embed support

You can use `go:embed` file system for config files

```go

//go:embed config
var fs embed.FS

confucius.Load(&cfg,
  confucius.EmbedFS(fs),
)
```

### Logger support

You can integrate with your log library confucius's logs

```go
confucius.Load(&cfg,
  confucius.Logger(
    confucius.SetLevel(confucius.ErrorLevel),
    confucius.SetOutput(os.Stdout),
    confucius.Callback(func(level confucius.LogLevel, message, file string, line int) {
      // log callback for logrus, zap or etc...
    }),
  ),
)
```

## Environment

Need to additionally fill fields from the environment? It's as simple as:

```go
confucius.Load(&cfg, confucius.UseEnv("MYAPP"))
```

## Usage

See usage [examples](/examples).

## Documentation

See [go.dev](https://pkg.go.dev/github.com/netologist/confucius?tab=doc) for detailed documentation.

## Contributing

PRs are welcome! Please explain your motivation for the change in your PR and ensure your change is properly tested and documented.
