package config

import (
	"fmt"
	"time"

	"github.com/hasanozgan/confucius"
)

type Config struct {
	App struct {
		Environment string `conf:"environment" validate:"required"`
	} `conf:"app"`
	Server struct {
		Host         string        `conf:"host" default:"0.0.0.0"`
		Port         int           `conf:"port" default:"80"`
		ReadTimeout  time.Duration `conf:"read_timeout" default:"30s"`
		WriteTimeout time.Duration `conf:"write_timeout" default:"30s"`
	} `conf:"server"`
	Logger struct {
		Level string `conf:"level" default:"info"`
	} `conf:"logger"`
	Certificate struct {
		Version    int       `conf:"version"`
		DNSNames   []string  `conf:"dns_names" default:"[kkyr,kkyr.io]"`
		Expiration time.Time `conf:"expiration" validate:"required"`
	} `conf:"certificate"`
}

func ExampleLoad() {
	var cfg Config
	err := confucius.Load(&cfg, confucius.TimeLayout("2006-01-02"))
	if err != nil {
		panic(err)
	}

	fmt.Println(cfg.App.Environment)
	fmt.Println(cfg.Server.Host)
	fmt.Println(cfg.Server.Port)
	fmt.Println(cfg.Server.ReadTimeout)
	fmt.Println(cfg.Server.WriteTimeout)
	fmt.Println(cfg.Logger.Level)
	fmt.Println(cfg.Certificate.Version)
	fmt.Println(cfg.Certificate.DNSNames)
	fmt.Println(cfg.Certificate.Expiration.Format("2006-01-02"))

	// Output:
	// dev
	// 0.0.0.0
	// 443
	// 1m0s
	// 30s
	// debug
	// 1
	// [kkyr kkyr.io]
	// 2020-12-01
}
