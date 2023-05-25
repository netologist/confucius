package required

import (
	"fmt"
	"time"

	"github.com/netologist/confucius"
)

type Config struct {
	Cache struct {
		Size            int           `conf:"size" default:"10000"`
		CleanupInterval time.Duration `conf:"cleanup_interval" validate:"required"`
	} `conf:"cache"`
	Tags []string `conf:"tags" validate:"required"`
}

func ExampleLoad() {
	var cfg Config
	err := confucius.Load(&cfg, confucius.File("config.json"), confucius.Tag("conf"))
	fmt.Println(err)

	// Output:
	// cache.cleanup_interval: required validation failed, tags: required validation failed
}
