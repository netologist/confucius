package confucius

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type Pod struct {
	APIVersion string `conf:"apiVersion" default:"v1"`
	Kind       string `conf:"kind" validate:"required"`
	Metadata   struct {
		Name           string        `conf:"name"`
		Environments   []string      `conf:"environments" default:"[dev,staging,prod]"`
		Master         bool          `conf:"master" validate:"required"`
		MaxPercentUtil *float64      `conf:"maxPercentUtil" default:"0.5"`
		Retry          time.Duration `conf:"retry" default:"10s"`
	} `conf:"metadata"`
	Spec Spec `conf:"spec"`
}

type Spec struct {
	Containers []Container `conf:"containers"`
	Volumes    []*Volume   `conf:"volumes"`
}

type Container struct {
	Name      string   `conf:"name" validate:"required"`
	Image     string   `conf:"image" validate:"required"`
	Command   []string `conf:"command"`
	Env       []Env    `conf:"env"`
	Ports     []Port   `conf:"ports"`
	Resources struct {
		Limits struct {
			CPU string `conf:"cpu"`
		} `conf:"limits"`
		Requests *struct {
			Memory string  `conf:"memory" default:"64Mi"`
			CPU    *string `conf:"cpu" default:"250m"`
		}
	} `conf:"resources"`
	VolumeMounts []VolumeMount `conf:"volumeMounts"`
}

type Env struct {
	Name  string `conf:"name"`
	Value string `conf:"value"`
}

type Port struct {
	ContainerPort int `conf:"containerPort" validate:"required"`
}

type VolumeMount struct {
	MountPath string `conf:"mountPath" validate:"required"`
	Name      string `conf:"name" validate:"required"`
}

type Volume struct {
	Name      string     `conf:"name" validate:"required"`
	ConfigMap *ConfigMap `conf:"configMap"`
}

type ConfigMap struct {
	Name  string `conf:"name" validate:"required"`
	Items []Item `conf:"items" validate:"required"`
}

type Item struct {
	Key  string `conf:"key" validate:"required"`
	Path string `conf:"path" validate:"required"`
}

func validPodConfig() Pod {
	var pod Pod

	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Metadata.Name = "redis"
	pod.Metadata.Environments = []string{"dev", "staging", "prod"}
	pod.Metadata.Master = true
	pod.Metadata.Retry = 10 * time.Second
	percentUtil := 0.5
	pod.Metadata.MaxPercentUtil = &percentUtil
	pod.Spec.Containers = []Container{
		{
			Name:  "redis",
			Image: "redis:5.0.4",
			Command: []string{
				"redis-server",
				"/redis-master/redis.conf",
			},
			Env: []Env{
				{
					Name:  "MASTER",
					Value: "true",
				},
			},
			Ports: []Port{
				{ContainerPort: 6379},
			},
			VolumeMounts: []VolumeMount{
				{
					MountPath: "/redis-master-data",
					Name:      "data",
				},
				{
					MountPath: "/redis-master",
					Name:      "config",
				},
			},
		},
	}
	pod.Spec.Containers[0].Resources.Limits.CPU = "0.1"
	pod.Spec.Volumes = []*Volume{
		{Name: "data"},
		{
			Name: "config",
			ConfigMap: &ConfigMap{
				Name: "example-redis-config",
				Items: []Item{
					{
						Key:  "redis-config",
						Path: "redis.conf",
					},
				},
			},
		},
	}

	return pod
}

//go:embed testdata/embed
var embedFS embed.FS

func Test_confucius_Load(t *testing.T) {
	for _, f := range []string{"pod.yaml", "pod.json", "pod.toml"} {
		t.Run(f, func(t *testing.T) {
			var cfg Pod
			err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			want := validPodConfig()

			if !reflect.DeepEqual(want, cfg) {
				t.Errorf("\nwant %+v\ngot %+v", want, cfg)
			}
		})
	}
}

func Test_confucius_findFiles(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var cfg Pod
		err := Load(&cfg,
			File("pod.yaml"),
			Dirs(filepath.Join("testdata", "valid")),
			EmbedFS(embedFS),
		)

		if err != nil {
			t.Fatalf("expected err: %+v", err)
		}
	})

	t.Run("main file not found", func(t *testing.T) {
		var cfg Pod
		err := Load(&cfg,
			File("not-found.yaml"),
			Dirs(filepath.Join("testdata", "valid")),
			EmbedFS(embedFS),
		)

		if err == nil {
			t.Fatalf("expected err")
		}
	})

	t.Run("profile file not found", func(t *testing.T) {
		var cfg Pod
		err := Load(&cfg,
			File("pod.yaml"),
			Dirs(filepath.Join("testdata", "valid")),
			EmbedFS(embedFS),
			Profiles("abc"),
		)

		if err == nil {
			t.Fatalf("expected err")
		}
	})

	t.Run("no file not found error when using string", func(t *testing.T) {
		var cfg struct {
			name string `conf:"name"`
		}
		err := Load(&cfg,
			String(`name: "john doe"`, DecoderYaml),
		)

		if err != nil {
			t.Fatalf("expected err: %+v", err)
		}
	})
}

func Test_confucius_findLocalFiles(t *testing.T) {
	conf := defaultConfucius()
	conf.filename = "pod.yaml"
	conf.dirs = []string{filepath.Join("testdata", "valid")}

	acc := conf.findLocalFiles()
	if !reflect.DeepEqual(acc, []string{"#local:#main=testdata/valid/pod.yaml"}) {
		t.Fatalf("inconsistent: %+v", acc)
	}
}

func Test_confucius_findEmbedFiles(t *testing.T) {
	conf := defaultConfucius()
	conf.useEmbedFS = true
	conf.embedFS = embedFS
	conf.filename = "pod.yaml"

	if acc, err := conf.findEmbedFiles(); err != nil {
		t.Fatalf("expected err: %+v", err)
	} else if !reflect.DeepEqual(acc, []string{"#embed:#main=testdata/embed/pod.yaml"}) {
		t.Fatalf("inconsistent: %+v", acc)
	}
}

func Test_confucius_fileExists(t *testing.T) {
	conf := defaultConfucius()

	t.Run("main", func(t *testing.T) {
		if tag := conf.fileExists("config.yaml"); tag != "#main" {
			t.Fatal("main not found")
		}
	})

	t.Run("profiles", func(t *testing.T) {
		conf.profiles = []string{"e2e"}

		if tag := conf.fileExists("config.e2e.yaml"); tag != "#profile_00_e2e" {
			t.Fatal("not found in profiles")
		}
	})

	t.Run("profiles with order", func(t *testing.T) {
		conf.profiles = []string{"e2e", "uat"}

		if tag := conf.fileExists("config.e2e.yaml"); tag != "#profile_00_e2e" {
			t.Fatal("not found in profiles")
		}

		if tag := conf.fileExists("config.uat.yaml"); tag != "#profile_01_uat" {
			t.Fatal("not found in profiles")
		}
	})

	t.Run("not found in main and profiles", func(t *testing.T) {
		if tag := conf.fileExists("pod.yaml"); tag != "" {
			t.Fatal("file not found")
		}
	})
}

func Test_confucius_walkEmbedDir(t *testing.T) {
	conf := defaultConfucius()
	conf.useEmbedFS = true
	conf.embedFS = embedFS
	conf.filename = "pod.yaml"
	conf.profiles = []string{"dev", "e2e"}

	accumulator := []string{}
	found := map[string]bool{}

	t.Run("success", func(t *testing.T) {
		err := conf.walkEmbedDir(&accumulator, found, ".")

		if err != nil {
			t.Fatalf("not expected: %+v", err)
		}

		expected := []string{
			"#embed:#main=testdata/embed/pod.yaml",
			"#embed:#profile_01_e2e=testdata/embed/pod.e2e.yaml",
		}

		if !reflect.DeepEqual(accumulator, expected) {
			t.Fatal("accumulator not match")
		}
	})

	t.Run("when embed file is not read", func(t *testing.T) {
		err := conf.walkEmbedDir(&accumulator, found, "bad-path")
		if err == nil {
			t.Fatal("not expected")
		}
	})
}

func Test_confucius_initExpectedConfigFiles(t *testing.T) {
	conf := defaultConfucius()
	conf.profiles = []string{"e2e", "dev", "uat"}

	if len(conf.expectedConfigFiles) != 0 {
		t.Fatal("it should empty")
	}

	conf.initExpectedConfigFiles()

	if len(conf.expectedConfigFiles) != 4 {
		t.Fatal("unexpected file count")
	}

	if !reflect.DeepEqual(conf.expectedConfigFiles, []string{
		"config.yaml", "config.e2e.yaml", "config.dev.yaml", "config.uat.yaml",
	}) {
		t.Fatalf("inconsistent config files %+v", conf.expectedConfigFiles)
	}
}

func Test_confucius_removeFromExpectedList(t *testing.T) {
	conf := defaultConfucius()
	conf.initExpectedConfigFiles()

	conf.removeFromExpectedList("config.yaml")

	if !reflect.DeepEqual(conf.expectedConfigFiles, []string{}) {
		t.Fatalf("inconsistent config files %+v", conf.expectedConfigFiles)
	}
}

func Test_confucius_decodeEmbedFile(t *testing.T) {
	conf := defaultConfucius()
	conf.filename = "pod.yaml"
	conf.useEmbedFS = true
	conf.embedFS = embedFS

	t.Run("when not found", func(t *testing.T) {
		if _, err := conf.decodeEmbedFile("testdata/embed/pod.yaml"); err != nil {
			t.Fatalf("unexpected error, %+v", err)
		}
	})

	t.Run("when not found", func(t *testing.T) {
		if _, err := conf.decodeEmbedFile("abc.yaml"); err == nil {
			t.Fatal("unexpected error")
		}
	})
}

func Test_confucius_decodeFiles(t *testing.T) {
	// conf := defaultConfucius()
	// t.Run("when embedded file not found", func(t *testing.T) {
	// 	// conf.decodeFiles([]string{"embed:"})

	// })
}

func Test_confucius_replaceEnvironments(t *testing.T) {
	os.Setenv("FOO", "XXX")
	os.Setenv("BAR", "YYY")

	tests := []struct {
		name     string
		text     string
		want     string
		hasError bool
	}{
		{name: "environment with default value", text: "/x/y/${BAZ:a}", want: "/x/y/a"},
		{name: "no environment tag", text: "/x/y/z", want: "/x/y/z"},
		{name: "from environment", text: "/x/y/${FOO}", want: "/x/y/XXX"},
		{name: "environment when is not set", text: "/x/y/${BAZ}", want: "/x/y/"},
		{name: "environment when is not set and default value is missing", text: "/x/y/${BAZ:}", want: "/x/y/"},
		{name: "environment name is missing", text: "/x/y/${}", hasError: true},
		{name: "multiple environment names", text: "/x/y/${FOO}/z/${BAR}", want: "/x/y/XXX/z/YYY"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if result, err := replaceEnvironments(test.text); err != nil {
				if test.hasError && err == nil {
					t.Error("not expected")
				}
			} else if test.want != result {
				t.Error("not expected")
			}
		})
	}
}

func Test_confucius_Load_If_Env_Set_In_Conf_File(t *testing.T) {
	os.Setenv("POD_NAME", "ehcache")
	for _, f := range []string{"pod.yaml", "pod.json", "pod.toml"} {
		t.Run(f, func(t *testing.T) {
			var cfg Pod
			err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			want := validPodConfig()
			want.Metadata.Name = "ehcache"

			if !reflect.DeepEqual(want, cfg) {
				t.Errorf("\nwant %+v\ngot %+v", want, cfg)
			}
		})
	}
}

func Test_confucius_Load_FileNotFound(t *testing.T) {
	confucius := defaultConfucius()
	confucius.filename = "abrakadabra"
	var cfg Pod
	err := confucius.Load(&cfg)
	if err == nil {
		t.Fatalf("expected err")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("expected err %v, got %v", ErrFileNotFound, err)
	}
}

func Test_confucius_Load_NonStructPtr(t *testing.T) {
	cfg := struct {
		X int
	}{}
	confucius := defaultConfucius()
	err := confucius.Load(cfg)
	if err == nil {
		t.Fatalf("confucius.Load() returned nil error")
	}
	if !strings.Contains(err.Error(), "pointer") {
		t.Errorf("expected struct pointer err, got %v", err)
	}
}

func Test_confucius_Load_Required(t *testing.T) {
	for _, f := range []string{"pod.yaml", "pod.json", "pod.toml"} {
		t.Run(f, func(t *testing.T) {
			var cfg Pod
			err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "invalid")))
			if err == nil {
				t.Fatalf("expected err")
			}

			want := []string{
				"kind",
				"metadata.master",
				"spec.containers[0].image",
				"spec.volumes[0].configMap.items",
				"spec.volumes[1].name",
			}

			fieldErrs := err.(fieldErrors)

			if len(want) != len(fieldErrs) {
				t.Fatalf("\nwant len(fieldErrs) == %d, got %d\nerrs: %+v\n", len(want), len(fieldErrs), fieldErrs)
			}

			for _, field := range want {
				if _, ok := fieldErrs[field]; !ok {
					t.Errorf("want %s in fieldErrs, got %+v", field, fieldErrs)
				}
			}
		})
	}
}

func Test_confucius_Load_Defaults(t *testing.T) {
	t.Run("non-zero values are not overridden", func(t *testing.T) {
		for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
			t.Run(f, func(t *testing.T) {
				type Server struct {
					Host   string `conf:"host" default:"127.0.0.1"`
					Ports  []int  `conf:"ports" default:"[80,443]"`
					Logger struct {
						LogLevel   string `conf:"log_level" default:"info"`
						Production bool   `conf:"production"`
						Metadata   struct {
							Keys []string `conf:"keys" default:"[ts]"`
						}
					}
					Application struct {
						BuildDate time.Time `conf:"build_date" default:"2020-01-01T12:00:00Z"`
					}
				}

				var want Server
				want.Host = "0.0.0.0"
				want.Ports = []int{80, 443}
				want.Logger.LogLevel = "debug"
				want.Logger.Production = false
				want.Logger.Metadata.Keys = []string{"ts"}
				want.Application.BuildDate = time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

				var cfg Server
				err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}

				if !reflect.DeepEqual(want, cfg) {
					t.Errorf("\nwant %+v\ngot %+v", want, cfg)
				}
			})
		}
	})

	t.Run("bad defaults reported as errors", func(t *testing.T) {
		for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
			t.Run(f, func(t *testing.T) {
				type Server struct {
					Host   string `conf:"host" default:"127.0.0.1"`
					Ports  []int  `conf:"ports" default:"[80,not-a-port]"`
					Logger struct {
						LogLevel string `conf:"log_level" default:"info"`
						Metadata struct {
							Keys []string `conf:"keys" validate:"required"`
						}
					}
					Application struct {
						BuildDate time.Time `conf:"build_date" default:"not-a-time"`
					}
				}

				var cfg Server
				err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
				if err == nil {
					t.Fatalf("expected err")
				}

				want := []string{
					"ports",
					"Logger.Metadata.keys",
					"Application.build_date",
				}

				fieldErrs := err.(fieldErrors)

				if len(want) != len(fieldErrs) {
					t.Fatalf("\nlen(fieldErrs) != %d\ngot %+v\n", len(want), fieldErrs)
				}

				for _, field := range want {
					if _, ok := fieldErrs[field]; !ok {
						t.Errorf("want %s in fieldErrs, got %+v", field, fieldErrs)
					}
				}
			})
		}
	})
}

func Test_confucius_Load_RequiredAndDefaults(t *testing.T) {
	for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
		t.Run(f, func(t *testing.T) {
			type Server struct {
				Host   string `conf:"host" default:"127.0.0.1"`
				Ports  []int  `conf:"ports" validate:"required"`
				Logger struct {
					LogLevel string `conf:"log_level" validate:"required"`
					Metadata struct {
						Keys []string `conf:"keys" validate:"required"`
					}
				}
				Application struct {
					BuildDate time.Time `conf:"build_date" default:"2020-01-01T12:00:00Z"`
				}
			}

			var cfg Server
			err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
			if err == nil {
				t.Fatalf("expected err")
			}

			want := []string{
				"ports",
				"Logger.Metadata.keys",
			}

			fieldErrs := err.(fieldErrors)

			if len(want) != len(fieldErrs) {
				t.Fatalf("\nlen(fieldErrs) != %d\ngot %+v\n", len(want), fieldErrs)
			}

			for _, field := range want {
				if _, ok := fieldErrs[field]; !ok {
					t.Errorf("want %s in fieldErrs, got %+v", field, fieldErrs)
				}
			}
		})
	}
}

func Test_confucius_Load_WithOptions(t *testing.T) {
	for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
		t.Run(f, func(t *testing.T) {
			type Server struct {
				Host   string `custom:"host" default:"127.0.0.1"`
				Ports  []int  `custom:"ports" default:"[80,443]"`
				Logger struct {
					LogLevel string `custom:"log_level"`
					Metadata struct {
						Keys []string `custom:"keys" default:"ts"`
						Tag  string   `custom:"tag" validate:"required"`
					}
				}
				Cache struct {
					CleanupInterval time.Duration `custom:"cleanup_interval" validate:"required"`
					FillThreshold   float32       `custom:"threshold" default:"0.9"`
				}
				Application struct {
					BuildDate time.Time `custom:"build_date" default:"12-25-2012"`
					Version   int
				}
			}

			os.Clearenv()
			setenv(t, "MYAPP_LOGGER_METADATA_TAG", "errorLogger")
			setenv(t, "MYAPP_LOGGER_LOG_LEVEL", "error")
			setenv(t, "MYAPP_APPLICATION_VERSION", "1")
			setenv(t, "MYAPP_CACHE_CLEANUP_INTERVAL", "5m")
			setenv(t, "MYAPP_CACHE_THRESHOLD", "0.85")

			var want Server
			want.Host = "0.0.0.0"
			want.Ports = []int{80, 443}
			want.Logger.LogLevel = "error"
			want.Logger.Metadata.Keys = []string{"ts"}
			want.Application.BuildDate = time.Date(2012, 12, 25, 0, 0, 0, 0, time.UTC)
			want.Logger.Metadata.Tag = "errorLogger"
			want.Application.Version = 1
			want.Cache.CleanupInterval = 5 * time.Minute
			want.Cache.FillThreshold = 0.85

			var cfg Server

			err := Load(&cfg,
				File(f),
				Dirs(filepath.Join("testdata", "valid")),
				Tag("custom"),
				TimeLayout("01-02-2006"),
				UseEnv("myapp"),
			)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if !reflect.DeepEqual(want, cfg) {
				t.Errorf("\nwant %+v\ngot %+v", want, cfg)
			}
		})
	}
}

func Test_confucius_Load_Server_If_Env_Set_In_Conf_File(t *testing.T) {
	os.Setenv("SERVICE_HOST", "192.168.0.128")
	for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
		t.Run(f, func(t *testing.T) {
			type Server struct {
				Host string `conf:"host"`
			}

			var cfg Server
			err := Load(&cfg, File(f), Dirs(filepath.Join("testdata", "valid")))
			if err != nil {
				t.Fatalf("expected err")
			}

			want := Server{Host: "192.168.0.128"}

			if !reflect.DeepEqual(want, cfg) {
				t.Errorf("\nwant %+v\ngot %+v", want, cfg)
			}
		})
	}
}

func Test_confucius_Load_String_Conf_File(t *testing.T) {
	type Server struct {
		Host string `conf:"host"`
	}

	var cfg Server
	err := Load(&cfg, String(`host: "127.0.0.1"`, DecoderYaml))
	if err != nil {
		t.Fatalf("expected err: %v", err)
	}

	want := Server{Host: "127.0.0.1"}

	if !reflect.DeepEqual(want, cfg) {
		t.Errorf("\nwant %+v\ngot %+v", want, cfg)
	}
}

func Test_confucius_Load_Environment_Conf_File(t *testing.T) {
	os.Setenv("MYAPP_HOST", "127.0.0.1")

	type Server struct {
		Host string `conf:"host"`
	}
	var cfg Server
	err := Load(&cfg, UseEnv("myapp"))
	if err != nil {
		t.Fatalf("expected err: %v", err)
	}

	want := Server{Host: "127.0.0.1"}

	if !reflect.DeepEqual(want, cfg) {
		t.Errorf("\nwant %+v\ngot %+v", want, cfg)
	}
}

func Test_confucius_Load_And_Merge_String_With_Conf_File(t *testing.T) {
	os.Unsetenv("SERVICE_HOST")
	type Server struct {
		Host string `conf:"host"`
	}

	var cfg Server
	err := Load(&cfg,
		String(`host: "127.0.0.1"`, DecoderYaml),
		File("server.yaml"),
		Dirs(filepath.Join("testdata", "valid")),
	)
	if err != nil {
		t.Fatalf("expected err: %v", err)
	}

	want := Server{Host: "0.0.0.0"}

	if !reflect.DeepEqual(want, cfg) {
		t.Errorf("\nwant %+v\ngot %+v", want, cfg)
	}
}

func Test_confucius_Load_And_Merge_String_With_Environment_Variable(t *testing.T) {
	os.Setenv("SERVICE_HOST", "192.168.0.128")
	type Server struct {
		Host string `conf:"host"`
	}

	var cfg Server
	err := Load(&cfg,
		String(`host: "127.0.0.1"`, DecoderYaml),
		File("server.yaml"),
		Dirs(filepath.Join("testdata", "valid")),
	)
	if err != nil {
		t.Fatalf("expected err: %v", err)
	}

	want := Server{Host: "192.168.0.128"}

	if !reflect.DeepEqual(want, cfg) {
		t.Errorf("\nwant %+v\ngot %+v", want, cfg)
	}
}

func Test_confucius_Return_Error_WhenLoad_Reader_Conf_File(t *testing.T) {
	type Server struct {
		Host string `conf:"host"`
	}

	byteConfig := strings.NewReader(`host: "127.0.0.1"`)
	var cfg Server
	err := Load(&cfg, Reader(byteConfig, DecoderJSON))
	if err == nil {
		t.Fatalf("it should be return error")
	}
}

func Test_confucius_Load_Server_With_Profile(t *testing.T) {
	for _, f := range []string{"server.yaml", "server.json", "server.toml"} {
		t.Run(f, func(t *testing.T) {
			type Server struct {
				Host   string `conf:"host"`
				Logger struct {
					LogLevel string `conf:"log_level" default:"info"`
					Appender string `conf:"appender"`
				}
				Replicas []string
			}

			var cfg Server
			err := Load(&cfg,
				File(f),
				Dirs(filepath.Join("testdata", "valid")),
				Profiles("test"),
				ProfileLayout("config.test.yaml"),
			)
			if err != nil {
				t.Fatalf("expected err %v", err)
			}

			want := Server{Host: "192.168.0.256"}
			want.Logger.LogLevel = "error"
			want.Logger.Appender = "file"
			want.Replicas = []string{"xyz"}

			if !reflect.DeepEqual(want, cfg) {
				t.Errorf("\nwant %+v\ngot %+v", want, cfg)
			}
		})
	}
}

func Test_confucius_Load_Server_With_Profile_When_Config_Is_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		profile string
	}{
		{name: "profile file is not found", file: "pod.yaml", profile: "test"},
		{name: "config file when bad format", file: "bad.yaml", profile: "test"},
		{name: "profile file when bad format", file: "pod.yaml", profile: "bad"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := struct{}{}
			err := Load(&cfg,
				File(test.file),
				Dirs(
					filepath.Join("testdata", "valid"),
					filepath.Join("testdata", "invalid"),
				),
				Profiles(test.profile),
				ProfileLayout("config-test.yaml"),
			)

			if err == nil {
				t.Fatalf("expected err %v", err)
			}
		})
	}
}

func Test_confucius_decodeFile(t *testing.T) {
	confucius := defaultConfucius()

	for _, f := range []string{"bad.yaml", "bad.json", "bad.toml"} {
		t.Run(f, func(t *testing.T) {
			file := filepath.Join("testdata", "invalid", f)
			if !fileExists(file) {
				t.Fatalf("test file %s does not exist", file)
			}
			_, err := confucius.decodeFile(file)
			if err == nil {
				t.Errorf("received nil error")
			}
		})
	}

	t.Run("unsupported file extension", func(t *testing.T) {
		file := filepath.Join("testdata", "invalid", "list.hcl")
		if !fileExists(file) {
			t.Fatalf("test file %s does not exist", file)
		}
		_, err := confucius.decodeFile(file)
		if err == nil {
			t.Fatal("received nil error")
		}
		if !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("err == %v, expected unsupported file extension", err)
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		_, err := confucius.decodeFile("casperthefriendlygho.st")
		if err == nil {
			t.Fatal("received nil error")
		}
	})
}

func Test_confucius_decodeMap(t *testing.T) {
	confucius := defaultConfucius()
	confucius.tag = "conf"

	m := decodedObject{
		"log_level": "debug",
		"severity":  "5",
		"server": decodedObject{
			"ports":  []int{443, 80},
			"secure": 1,
		},
	}

	var cfg struct {
		Level    string `conf:"log_level"`
		Severity int    `conf:"severity" validate:"required"`
		Server   struct {
			Ports  []string `conf:"ports" default:"[443]"`
			Secure bool
		} `conf:"server"`
	}

	err := confucius.decodeMap(m, &cfg)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if cfg.Level != "debug" {
		t.Errorf("cfg.Level: want %s, got %s", "debug", cfg.Level)
	}

	if cfg.Severity != 5 {
		t.Errorf("cfg.Severity: want %d, got %d", 5, cfg.Severity)
	}

	if reflect.DeepEqual([]int{443, 80}, cfg.Server.Ports) {
		t.Errorf("cfg.Server.Ports: want %+v, got %+v", []int{443, 80}, cfg.Server.Ports)
	}

	if cfg.Server.Secure == false {
		t.Error("cfg.Server.Secure == false")
	}
}

func Test_confucius_processCfg(t *testing.T) {
	t.Run("slice elements set by env", func(t *testing.T) {
		confucius := defaultConfucius()
		confucius.tag = "conf"
		confucius.useEnv = true

		os.Clearenv()
		setenv(t, "A_0_B", "b0")
		setenv(t, "A_1_B", "b1")
		setenv(t, "A_0_C", "9000")

		cfg := struct {
			A []struct {
				B string `validate:"required"`
				C int    `default:"5"`
			}
		}{}
		cfg.A = []struct {
			B string `validate:"required"`
			C int    `default:"5"`
		}{{B: "boo"}, {B: "boo"}}

		err := confucius.processCfg(&cfg)
		if err != nil {
			t.Fatalf("processCfg() returned unexpected error: %v", err)
		}
		if cfg.A[0].B != "b0" {
			t.Errorf("cfg.A[0].B == %s, expected %s", cfg.A[0].B, "b0")
		}
		if cfg.A[1].B != "b1" {
			t.Errorf("cfg.A[1].B == %s, expected %s", cfg.A[1].B, "b1")
		}
		if cfg.A[0].C != 9000 {
			t.Errorf("cfg.A[0].C == %d, expected %d", cfg.A[0].C, 9000)
		}
		if cfg.A[1].C != 5 {
			t.Errorf("cfg.A[1].C == %d, expected %d", cfg.A[1].C, 5)
		}
	})

	t.Run("embedded struct set by env", func(t *testing.T) {
		confucius := defaultConfucius()
		confucius.useEnv = true
		confucius.tag = "conf"

		type A struct {
			B string
		}
		type C struct {
			D *int
		}
		type F struct {
			A
			C `conf:"cc"`
		}
		cfg := F{}

		os.Clearenv()
		setenv(t, "A_B", "embedded")
		setenv(t, "CC_D", "7")

		err := confucius.processCfg(&cfg)
		if err != nil {
			t.Fatalf("processCfg() returned unexpected error: %v", err)
		}
		if cfg.A.B != "embedded" {
			t.Errorf("cfg.A.B == %s, expected %s", cfg.A.B, "embedded")
		}
		if *cfg.C.D != 7 {
			t.Errorf("cfg.C.D == %d, expected %d", *cfg.C.D, 7)
		}
	})
}

func Test_confucius_processField(t *testing.T) {
	confucius := defaultConfucius()
	confucius.tag = "conf"

	t.Run("field with default", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" default:"10"`
		}{}
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err != nil {
			t.Fatalf("processField() returned unexpected error: %v", err)
		}
		if cfg.X != 10 {
			t.Errorf("cfg.X == %d, expected %d", cfg.X, 10)
		}
	})

	t.Run("field with default does not overwrite", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" default:"10"`
		}{}
		cfg.X = 5
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err != nil {
			t.Fatalf("processField() returned unexpected error: %v", err)
		}
		if cfg.X != 5 {
			t.Errorf("cfg.X == %d, expected %d", cfg.X, 5)
		}
	})

	t.Run("field with bad default", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" default:"not-an-int"`
		}{}
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err == nil {
			t.Fatalf("processField() returned nil error")
		}
	})

	t.Run("field with required", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" validate:"required"`
		}{}
		cfg.X = 10
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err != nil {
			t.Fatalf("processField() returned unexpected error: %v", err)
		}
		if cfg.X != 10 {
			t.Errorf("cfg.X == %d, expected %d", cfg.X, 10)
		}
	})

	t.Run("field with required error", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" validate:"required"`
		}{}
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err == nil {
			t.Fatalf("processField() returned nil error")
		}
	})

	t.Run("field with default and required", func(t *testing.T) {
		cfg := struct {
			X int `conf:"y" default:"10" validate:"required"`
		}{}
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err == nil {
			t.Fatalf("processField() expected error")
		}
	})

	t.Run("field overwritten by env", func(t *testing.T) {
		confucius := defaultConfucius()
		confucius.tag = "conf"
		confucius.useEnv = true
		confucius.envPrefix = "confucius"

		os.Clearenv()
		setenv(t, "CONFUCIUS_X", "MEN")

		cfg := struct {
			X string `conf:"x"`
		}{}
		cfg.X = "BOYS"
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err != nil {
			t.Fatalf("processField() returned unexpected error: %v", err)
		}
		if cfg.X != "MEN" {
			t.Errorf("cfg.X == %s, expected %s", cfg.X, "MEN")
		}
	})

	t.Run("field with bad env", func(t *testing.T) {
		confucius := defaultConfucius()
		confucius.tag = "conf"
		confucius.useEnv = true
		confucius.envPrefix = "confucius"

		os.Clearenv()
		setenv(t, "CONFUCIUS_I", "FIFTY")

		cfg := struct {
			I int
		}{}
		parent := &field{
			v:        reflect.ValueOf(&cfg).Elem(),
			t:        reflect.ValueOf(&cfg).Elem().Type(),
			sliceIdx: -1,
		}

		f := newStructField(parent, 0, confucius.tag)
		err := confucius.processField(f)
		if err == nil {
			t.Fatalf("processField() returned nil error")
		}
	})
}

func Test_confucius_setFromEnv(t *testing.T) {
	confucius := defaultConfucius()
	confucius.envPrefix = "confucius"

	var s string
	fv := reflect.ValueOf(&s)

	os.Clearenv()
	err := confucius.setFromEnv(fv, "config.string")
	if err != nil {
		t.Fatalf("setFromEnv() unexpected error: %v", err)
	}
	if s != "" {
		t.Fatalf("s modified to %s", s)
	}

	setenv(t, "CONFUCIUS_CONFIG_STRING", "goroutine")
	err = confucius.setFromEnv(fv, "config.string")
	if err != nil {
		t.Fatalf("setFromEnv() unexpected error: %v", err)
	}
	if s != "goroutine" {
		t.Fatalf("s == %s, expected %s", s, "goroutine")
	}
}

func Test_confucius_formatEnvKey(t *testing.T) {
	confucius := defaultConfucius()

	for _, tc := range []struct {
		key    string
		prefix string
		want   string
	}{
		{
			key:  "port",
			want: "PORT",
		},
		{
			key:    "server.host",
			prefix: "myapp",
			want:   "MYAPP_SERVER_HOST",
		},
		{
			key:  "loggers[0].log_level",
			want: "LOGGERS_0_LOG_LEVEL",
		},
		{
			key:  "nested[1].slice[2].twice",
			want: "NESTED_1_SLICE_2_TWICE",
		},
		{
			key:    "client.http.timeout",
			prefix: "auth_s",
			want:   "AUTH_S_CLIENT_HTTP_TIMEOUT",
		},
	} {
		t.Run(fmt.Sprintf("%s/%s", tc.prefix, tc.key), func(t *testing.T) {
			confucius.envPrefix = tc.prefix
			got := confucius.formatEnvKey(tc.key)
			if got != tc.want {
				t.Errorf("formatEnvKey() == %s, expected %s", got, tc.want)
			}
		})
	}
}

func Test_confucius_setDefaultValue(t *testing.T) {
	confucius := defaultConfucius()
	var b bool
	fv := reflect.ValueOf(&b).Elem()

	err := confucius.setDefaultValue(fv, "true")
	if err == nil {
		t.Fatalf("expected err")
	}
}

func Test_confucius_setValue(t *testing.T) {
	confucius := defaultConfucius()

	t.Run("nil ptr", func(t *testing.T) {
		var s *string
		fv := reflect.ValueOf(&s)

		err := confucius.setValue(fv, "bat")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if *s != "bat" {
			t.Fatalf("want %s, got %s", "bat", *s)
		}
	})

	t.Run("slice", func(t *testing.T) {
		var slice []int
		fv := reflect.ValueOf(&slice).Elem()

		err := confucius.setValue(fv, "5")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if !reflect.DeepEqual([]int{5}, slice) {
			t.Fatalf("want %+v, got %+v", []int{5}, slice)
		}
	})

	t.Run("int", func(t *testing.T) {
		var i int
		fv := reflect.ValueOf(&i).Elem()

		err := confucius.setValue(fv, "-8")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if i != -8 {
			t.Fatalf("want %d, got %d", -8, i)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()

		err := confucius.setValue(fv, "true")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if !b {
			t.Fatalf("want true")
		}
	})

	t.Run("bad bool", func(t *testing.T) {
		var b bool
		fv := reflect.ValueOf(&b).Elem()

		err := confucius.setValue(fv, "αλήθεια")
		if err == nil {
			t.Fatalf("returned nil err")
		}
	})

	t.Run("duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()

		err := confucius.setValue(fv, "5h")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if d.Hours() != 5 {
			t.Fatalf("want %v, got %v", 5*time.Hour, d)
		}
	})

	t.Run("bad duration", func(t *testing.T) {
		var d time.Duration
		fv := reflect.ValueOf(&d).Elem()

		err := confucius.setValue(fv, "5decades")
		if err == nil {
			t.Fatalf("expexted err")
		}
	})

	t.Run("uint", func(t *testing.T) {
		var i uint
		fv := reflect.ValueOf(&i).Elem()

		err := confucius.setValue(fv, "42")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if i != 42 {
			t.Fatalf("want %d, got %d", 42, i)
		}
	})

	t.Run("float", func(t *testing.T) {
		var f float32
		fv := reflect.ValueOf(&f).Elem()

		err := confucius.setValue(fv, "0.015625")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if f != 0.015625 {
			t.Fatalf("want %f, got %f", 0.015625, f)
		}
	})

	t.Run("bad float", func(t *testing.T) {
		var f float32
		fv := reflect.ValueOf(&f).Elem()

		err := confucius.setValue(fv, "-i")
		if err == nil {
			t.Fatalf("expected err")
		}
	})

	t.Run("string", func(t *testing.T) {
		var s string
		fv := reflect.ValueOf(&s).Elem()

		err := confucius.setValue(fv, "bat")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if s != "bat" {
			t.Fatalf("want %s, got %s", "bat", s)
		}
	})

	t.Run("time", func(t *testing.T) {
		var tme time.Time
		fv := reflect.ValueOf(&tme).Elem()

		err := confucius.setValue(fv, "2020-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		want, err := time.Parse(confucius.timeLayout, "2020-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("error parsing time: %v", err)
		}

		if !tme.Equal(want) {
			t.Fatalf("want %v, got %v", want, tme)
		}
	})

	t.Run("bad time", func(t *testing.T) {
		var tme time.Time
		fv := reflect.ValueOf(&tme).Elem()

		err := confucius.setValue(fv, "2020-Feb-01T00:00:00Z")
		if err == nil {
			t.Fatalf("expected err")
		}
	})

	t.Run("interface returns error", func(t *testing.T) {
		var i interface{}
		fv := reflect.ValueOf(i)

		err := confucius.setValue(fv, "empty")
		if err == nil {
			t.Fatalf("expected err")
		}
	})

	t.Run("struct returns error", func(t *testing.T) {
		s := struct{ Name string }{}
		fv := reflect.ValueOf(&s).Elem()

		err := confucius.setValue(fv, "foo")
		if err == nil {
			t.Fatalf("expected err")
		}
	})
}

func Test_confucius_setSlice(t *testing.T) {
	f := defaultConfucius()

	for _, tc := range []struct {
		Name      string
		InSlice   interface{}
		WantSlice interface{}
		Val       string
	}{
		{
			Name:      "ints",
			InSlice:   &[]int{},
			WantSlice: &[]int{5, 10, 15},
			Val:       "[5,10,15]",
		},
		{
			Name:      "ints-no-square-braces",
			InSlice:   &[]int{},
			WantSlice: &[]int{5, 10, 15},
			Val:       "5,10,15",
		},
		{
			Name:      "uints",
			InSlice:   &[]uint{},
			WantSlice: &[]uint{5, 10, 15, 20, 25},
			Val:       "[5,10,15,20,25]",
		},
		{
			Name:      "floats",
			InSlice:   &[]float32{},
			WantSlice: &[]float32{1.5, 1.125, -0.25},
			Val:       "[1.5,1.125,-0.25]",
		},
		{
			Name:      "strings",
			InSlice:   &[]string{},
			WantSlice: &[]string{"a", "b", "c", "d"},
			Val:       "[a,b,c,d]",
		},
		{
			Name:      "durations",
			InSlice:   &[]time.Duration{},
			WantSlice: &[]time.Duration{30 * time.Minute, 2 * time.Hour},
			Val:       "[30m,2h]",
		},
		{
			Name:    "times",
			InSlice: &[]time.Time{},
			WantSlice: &[]time.Time{
				time.Date(2019, 12, 25, 10, 30, 30, 0, time.UTC),
				time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			Val: "[2019-12-25T10:30:30Z,2020-01-01T00:00:00Z]",
		},
	} {
		t.Run(tc.Val, func(t *testing.T) {
			in := reflect.ValueOf(tc.InSlice).Elem()

			err := f.setSlice(in, tc.Val)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := reflect.ValueOf(tc.WantSlice).Elem()

			if !reflect.DeepEqual(want.Interface(), in.Interface()) {
				t.Fatalf("want %+v, got %+v", want, in)
			}
		})
	}

	t.Run("negative int into uint returns error", func(t *testing.T) {
		in := &[]uint{}
		val := "[-5]"

		err := f.setSlice(reflect.ValueOf(in).Elem(), val)
		if err == nil {
			t.Fatalf("expected err")
		}
	})
}

func Test_confucius_Load_Logger(t *testing.T) {
	writer := bytes.NewBufferString("")

	var cfg struct{}
	err := Load(&cfg,
		String(`{}`, DecoderJSON),
		Logger(
			SetLevel(DebugLevel),
			SetOutput(writer),
		),
	)

	if err != nil {
		t.Fail()
	}
}

func Test_confucius_Load_Logger_Callback(t *testing.T) {
	writer := bytes.NewBufferString("")

	var cfg struct{}
	err := Load(&cfg,
		String(`{}`, DecoderJSON),
		Logger(
			SetLevel(DebugLevel),
			Callback(func(level LogLevel, message, file string, line int) {
				writer.WriteString(message)
			}),
			SetOutput(os.Stdout),
		),
	)

	if err != nil {
		t.Fail()
	}

	if !strings.Contains(writer.String(), "confucius starting") {
		t.Fail()
	}
}

func setenv(t *testing.T, key, value string) {
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("os.Setenv() unexpected error: %v", err)
	}
}
