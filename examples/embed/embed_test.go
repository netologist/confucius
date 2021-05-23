package embed

import (
	"embed"
	"fmt"

	"github.com/hasanozgan/confucius"
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
var reference string

//go:embed embedded_config
var fs embed.FS

func ExampleLoad() {

	var cfg Config
	err := confucius.Load(&cfg,
		confucius.String(reference, confucius.DecoderYaml),
		confucius.Profiles("e2e", "uat"),
		confucius.ProfileLayout("test.yaml"),
		confucius.Dirs("local_config"),
		confucius.EmbedFS(fs),
	)
	if err == nil {
		fmt.Printf("%+v", cfg)
	} else {
		fmt.Print(err)
	}

	// Output:
	// {Database:{Host:db.uat.example.com Port:5432 Name:users Username:admin Password:secret} Kafka:{Host:[kafka.uat.example.com]}}
}
