package confucius

import (
	"io"
	"strings"
)

// Option configures how confucius loads the configuration.
type Option func(c *confucius)

// File returns an option that configures the filename that fig
// looks for to provide the config values.
//
// The name must include the extension of the file. Supported
// file types are `yaml`, `yml`, `json` and `toml`.
//
//   confucius.Load(&cfg, confucius.File("config.toml"))
//
// If this option is not used then confucius looks for a file with name `config.yaml`.
func File(name string) Option {
	return func(c *confucius) {
		c.filename = name
	}
}

// Reader returns an option that configure from reader for reference configuration.
func Reader(reader io.Reader, decoder Decoder) Option {
	return func(c *confucius) {
		c.useReader = true
		c.readerConfig = reader
		c.readerDecoder = decoder
	}
}

// String returns an option that configure from string for reference configuration.
func String(file string, decoder Decoder) Option {
	return Reader(strings.NewReader(strings.TrimSpace(file)), decoder)
}

// Dirs returns an option that configures the directories that confucius searches
// to find the configuration file.
//
// Directories are searched sequentially and the first one with a matching config file is used.
//
// This is useful when you don't know where exactly your configuration will be during run-time:
//
//   confucius.Load(&cfg, confucius.Dirs(".", "/etc/myapp", "/home/user/myapp"))
//
//
// If this option is not used then confucius looks in the directory it is run from.
func Dirs(dirs ...string) Option {
	return func(c *confucius) {
		c.dirs = dirs
	}
}

// Tag returns an option that configures the tag key that confucius uses
// when for the alt name struct tag key in fields.
//
//  confucius.Load(&cfg, confucius.Tag("config"))
//
// If this option is not used then confucius uses the tag `fig`.
func Tag(tag string) Option {
	return func(c *confucius) {
		c.tag = tag
	}
}

// TimeLayout returns an option that conmfigures the time layout that confucius uses when
// parsing a time in a config file or in the default tag for time.Time fields.
//
//   confucius.Load(&cfg, confucius.TimeLayout("2006-01-02"))
//
// If this option is not used then confucius parses times using `time.RFC3339` layout.
func TimeLayout(layout string) Option {
	return func(c *confucius) {
		c.timeLayout = layout
	}
}

// UseEnv returns an option that configures confucius to additionally load values
// from the environment, after it has loaded values from a config file.
//
//   confucius.Load(&cfg, confucius.UseEnv("my_app"))
//
// This is meant to be used in conjunction with loading from a file. There
// is no support to ONLY load from the environment.
//
// Fig looks for environment variables in the format PREFIX_FIELD_PATH or
// FIELD_PATH if prefix is empty. Prefix is capitalised regardless of what
// is provided. The field's path is formed by prepending its name with the
// names of all surrounding fields up to the root struct. If a field has
// an alternative name defined inside a struct tag then that name is
// preferred.
//
//   type Config struct {
//     Build    time.Time
//     LogLevel string `conf:"log_level"`
//     Server   struct {
//       Host string
//     }
//   }
//
// With the struct above and UseEnv("myapp") confucius would search for the following
// environment variables:
//
//   MYAPP_BUILD
//   MYAPP_LOG_LEVEL
//   MYAPP_SERVER_HOST
func UseEnv(prefix string) Option {
	return func(c *confucius) {
		c.useEnv = true
		c.envPrefix = prefix
	}
}

// Profiles returns an option that configures the profile key that confucius uses
//
//  confucius.Load(&cfg, confucius.UseProfile("test"))
//
// If this option is not used then confucius uses the tag `fig`.
func Profiles(profiles ...string) Option {
	return func(c *confucius) {
		c.profiles = profiles
	}
}

// ProfileLayout returns an option that configures the profile layout that confucius uses
//
//  confucius.Load(&cfg, confucius.UseProfileLayout("config-test.yaml"))
//
// If this option is not used then confucius uses the tag `fig`.
func ProfileLayout(layout string) Option {
	return func(c *confucius) {
		c.profileLayout = layout
	}
}
