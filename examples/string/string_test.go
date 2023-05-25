package embed

import (
	_ "embed"
	"fmt"

	"github.com/netologist/confucius"
)

type Config struct {
	Database struct {
		Host     string `conf:"host" validate:"required"`
		Port     int    `conf:"port"`
		Name     string `conf:"name" validate:"required"`
		Username string `conf:"username"`
		Password string `conf:"password"`
	}
	Kafka struct {
		Host []string `conf:"host" validate:"required"`
	}
}

//go:embed reference.yaml
var config string

func ExampleLoad() {

	var cfg Config
	if err := confucius.Load(&cfg, confucius.String(config, confucius.DecoderYaml)); err == nil {
		fmt.Printf("%+v", cfg)
	} else {
		fmt.Print(err)
	}

	// Output:
	// {Database:{Host:db.prod.example.com Port:5432 Name:orders Username:admin Password:S3cr3t-P455w0rd} Kafka:{Host:[kafka1.prod.example.com kafka2.prod.example.com]}}
}
