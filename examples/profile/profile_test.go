package profile

import (
	"fmt"
	"os"

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

func ExampleLoad() {
	var cfg Config
	if err := confucius.Load(&cfg); err == nil {
		fmt.Printf("%+v", cfg)
	} else {
		fmt.Println(err)
	}

	// Output:
	// {Database:{Host:db.prod.example.com Port:5432 Name:users Username:admin Password:S3cr3t-P455w0rd} Kafka:{Host:[kafka1.prod.example.com kafka2.prod.example.com]}}
}

func ExampleLoad_with_environment_in_config_file() {
	os.Setenv("DATABASE_NAME", "users-readonly")

	var cfg Config
	if err := confucius.Load(&cfg); err == nil {
		fmt.Printf("%+v", cfg)
	} else {
		fmt.Println(err)
	}

	// Output:
	// {Database:{Host:db.prod.example.com Port:5432 Name:users-readonly Username:admin Password:S3cr3t-P455w0rd} Kafka:{Host:[kafka1.prod.example.com kafka2.prod.example.com]}}
}

func ExampleLoad_with_multi_profile() {
	var cfg Config
	if err := confucius.Load(&cfg, confucius.Profiles("test", "integration"), confucius.ProfileLayout("config-test.yaml")); err == nil {
		fmt.Printf("%+v", cfg)
	}

	// Output:
	// {Database:{Host:db Port:5432 Name:users Username:admin Password:postgres} Kafka:{Host:[kafka]}}
}

func ExampleLoad_with_single_profile() {
	var cfg Config
	if err := confucius.Load(&cfg, confucius.Profiles("test"), confucius.ProfileLayout("config-test.yaml")); err == nil {
		fmt.Printf("%+v", cfg)
	}

	// Output:
	// {Database:{Host:sqlite:file.db Port:-1 Name:users Username: Password:} Kafka:{Host:[embedded:kafka]}}
}
