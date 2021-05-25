package confucius

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

const (
	// DefaultFilename is the default filename of the config file that confucius looks for.
	DefaultFilename = "config.yaml"
	// DefaultDir is the default directory that confucius searches in for the config file.
	DefaultDir = "."
	// DefaultTag is the default struct tag key that confucius uses to find the field's alt
	// name.
	DefaultTag = "conf"
	// DefaultTimeLayout is the default time layout that confucius uses to parse times.
	DefaultTimeLayout = time.RFC3339
	// DefaultProfileLayout represents default profile file layout.
	// You should use `config` for filename, `test` for profile, `yaml` for extension.
	// Example; config-test.yaml
	DefaultProfileLayout = "config.test.yaml"
	// MainFileIndicator is config file type indicator
	MainFileIndicator = "#main"
	// MainFileIndicator is config file type indicator
	ProfileFileIndicator = "#profile"
	// FileEmbedLocationInditcator is config file location indicator
	EmbedLocationInditcator = "#embed"
	// FileEmbedLocationInditcator is config file location indicator
	LocalLocationInditcator = "#local"
)

type decodedObject map[string]interface{}

func defaultConfucius() *confucius {
	return &confucius{
		filename:      DefaultFilename,
		dirs:          []string{DefaultDir},
		tag:           DefaultTag,
		timeLayout:    DefaultTimeLayout,
		profileLayout: DefaultProfileLayout,
	}
}

type confucius struct {
	useEnv              bool
	useReader           bool
	useEmbedFS          bool
	dirs                []string
	profiles            []string
	expectedConfigFiles []string
	filename            string
	tag                 string
	timeLayout          string
	envPrefix           string
	profileLayout       string
	readerConfig        io.Reader
	readerDecoder       Decoder
	embedFS             embed.FS
}

// Load reads a configuration file and loads it into the given struct. The
// parameter `cfg` must be a pointer to a struct.
//
// By default confucius looks for a file `config.yaml` in the current directory and
// uses the struct field tag `fig` for matching field names and validation.
// To alter this behaviour pass additional parameters as options.
//
// A field can be marked as required by adding a `required` key in the field's struct tag.
// If a required field is not set by the configuration file an error is returned.
//
//   type Config struct {
//     Env string `conf:"env" validate:"required"` // or just `validate:"required"`
//   }
//
// A field can be configured with a default value by adding a `default` key in the
// field's struct tag.
// If a field is not set by the configuration file then the default value is set.
//
//  type Config struct {
//    Level string `conf:"level" default:"info"` // or just `default:"info"`
//  }
//
// A single field may not be marked as both `required` and `default`.
func Load(cfg interface{}, options ...Option) error {
	confucius := defaultConfucius()

	for _, opt := range options {
		opt(confucius)
	}

	return confucius.Load(cfg)
}

func (c *confucius) Load(cfg interface{}) (err error) {
	if !isStructPtr(cfg) {
		return fmt.Errorf("cfg must be a pointer to a struct")
	}

	vals := make(decodedObject)
	if c.useReader {
		vals, err = c.decodeReader(c.readerConfig, c.readerDecoder)
		if err != nil {
			return err
		}
	}

	files, err := c.findFiles()
	if err != nil && !(c.useReader || c.useEnv) {
		return err
	}

	if vals, err = c.decodeFiles(files, vals); err != nil {
		return err
	}

	if err := c.decodeMap(vals, cfg); err != nil {
		return err
	}

	return c.processCfg(cfg)
}

func (c *confucius) findFiles() ([]string, error) {
	c.initExpectedConfigFiles()

	result := []string{}
	files, err := c.findEmbedFiles()
	if err != nil {
		return result, err
	}
	result = append(result, files...)
	result = append(result, c.findLocalFiles()...)

	if len(c.expectedConfigFiles) > 0 {
		return nil, fmt.Errorf("\"%s\" file(s) not found: %w",
			strings.Join(c.expectedConfigFiles, "\", \""),
			ErrFileNotFound,
		)
	}

	sort.StringSlice(result).Sort()
	return result, nil
}

func (c *confucius) findLocalFiles() (acc []string) {
	found := map[string]bool{}
	for _, dir := range c.dirs {
		path := filepath.Join(dir, c.filename)
		if fileExists(path) && !found[c.filename] {
			found[c.filename] = true
			c.removeFromExpectedList(c.filename)
			acc = append(acc,
				fmt.Sprintf("%s:%s=%s", LocalLocationInditcator, MainFileIndicator, path),
			)
		}

		for idx, profile := range c.profiles {
			profileName := c.profileFileName(profile)
			path := filepath.Join(dir, profileName)

			if fileExists(path) && !found[profileName] {
				found[profileName] = true
				c.removeFromExpectedList(profileName)
				acc = append(acc,
					fmt.Sprintf("%s:%s_%02d_%s=%s", LocalLocationInditcator, ProfileFileIndicator, idx, profile, path),
				)
			}
		}
	}
	return
}

func (c *confucius) findEmbedFiles() (acc []string, err error) {
	found := map[string]bool{}
	if c.useEmbedFS {
		err = c.walkEmbedDir(&acc, found, ".")
		if err != nil {
			return
		}
	}
	return
}

func (c confucius) fileExists(filename string) string {
	if c.filename == filename {
		return MainFileIndicator
	}

	for idx, profile := range c.profiles {
		profileName := c.profileFileName(profile)

		if profileName == filename {
			return fmt.Sprintf("%s_%02d_%s", ProfileFileIndicator, idx, profile)
		}
	}
	return ""
}

func (c *confucius) walkEmbedDir(accumulator *[]string, found map[string]bool, path string) error {
	entries, err := c.embedFS.ReadDir(path)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return !entries[i].IsDir()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			p := filepath.Join(path, entry.Name())
			if err := c.walkEmbedDir(accumulator, found, p); err != nil {
				return err
			}
		} else if tag := c.fileExists(entry.Name()); tag != "" && !found[entry.Name()] {
			found[entry.Name()] = true
			c.removeFromExpectedList(entry.Name())
			fullPath := path + "/" + entry.Name()
			*accumulator = append(*accumulator, fmt.Sprintf("%s:%s=%s", EmbedLocationInditcator, tag, fullPath))
		} else {
			log.Printf("file not found: %+v", entry.Name())
		}
	}
	return nil
}

func (c *confucius) initExpectedConfigFiles() {
	c.expectedConfigFiles = []string{c.filename}

	for _, profile := range c.profiles {
		c.expectedConfigFiles = append(c.expectedConfigFiles, c.profileFileName(profile))
	}
}

func (c *confucius) removeFromExpectedList(file string) {
	result := []string{}
	for _, expected := range c.expectedConfigFiles {
		if expected == file {
			continue
		}
		result = append(result, expected)
	}
	c.expectedConfigFiles = result
}

func (c *confucius) decodeEmbedFile(file string) (vals decodedObject, err error) {
	fd, err := c.embedFS.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	return c.decodeReader(fd, Decoder(filepath.Ext(file)))
}

func (c *confucius) decodeFiles(files []string, origin decodedObject) (vals decodedObject, err error) {
	vals = origin
	for _, file := range files {
		fileVals := decodedObject{}
		sections := strings.Split(file, "=")

		if strings.Contains(file, EmbedLocationInditcator) {
			fileVals, err = c.decodeEmbedFile(sections[1])
			if err != nil {
				return nil, err
			}
		}

		if strings.Contains(file, LocalLocationInditcator) {
			fileVals, err = c.decodeFile(sections[1])
			if err != nil {
				return nil, err
			}
		}

		if err := mergo.Merge(&vals, fileVals, mergo.WithOverride, mergo.WithTypeCheck); err != nil {
			return nil, err
		}
	}
	return vals, nil
}

func (c *confucius) profileFileName(profile string) string {
	filename := c.profileLayout
	parts := strings.Split(c.filename, ".")
	filename = strings.ReplaceAll(filename, "config", parts[0])
	filename = strings.ReplaceAll(filename, "test", profile)
	filename = strings.ReplaceAll(filename, "yaml", parts[1])
	return filename
}

// decodeFile reads the file and unmarshalls // it using a decoder based on the file extension.
func (c *confucius) decodeFile(file string) (decodedObject, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	return c.decodeReader(fd, Decoder(filepath.Ext(file)))
}

func (c *confucius) decodeReader(reader io.Reader, decoder Decoder) (decodedObject, error) {
	vals := make(decodedObject)

	switch decoder {
	case ".yaml", ".yml":
		if err := yaml.NewDecoder(reader).Decode(&vals); err != nil {
			return nil, err
		}
	case ".json":
		if err := json.NewDecoder(reader).Decode(&vals); err != nil {
			return nil, err
		}
	case ".toml":
		tree, err := toml.LoadReader(reader)
		if err != nil {
			return nil, err
		}
		for field, val := range tree.ToMap() {
			vals[field] = val
		}
	default:
		return nil, fmt.Errorf("unsupported file extension %s", filepath.Ext(c.filename))
	}

	return vals, nil
}

// decodeMap decodes a map of va// lues into result using the mapstructure library.
func (c *confucius) decodeMap(m decodedObject, result interface{}) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           result,
		TagName:          c.tag,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			fromEnvironmentHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToTimeHookFunc(c.timeLayout),
		),
	})
	if err != nil {
		return err
	}
	return dec.Decode(m)
}

func replaceEnvironments(str string) (result string, err error) {
	re := regexp.MustCompile(`\$\{(.*?|)\}`)
	result = str
	for _, match := range re.FindAllStringSubmatch(str, -1) {
		whole, value := match[0], match[1]
		if value == "" {
			return result, fmt.Errorf("environment name is missing")
		}

		s := strings.Split(value, ":")
		envName := s[0]
		if envValue, ok := os.LookupEnv(envName); ok {
			result = strings.ReplaceAll(result, whole, envValue)
		} else {
			defaultVal := ""
			if len(s) > 1 {
				defaultVal = s[1]
			}
			result = strings.ReplaceAll(result, whole, defaultVal)
		}
	}
	return result, err
}

func fromEnvironmentHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		return replaceEnvironments(data.(string))
	}
}

// processCfg processes a cfg struct after it has been loaded from
// the config file, by validating required fields and setting defaults
// where applicable.
func (c *confucius) processCfg(cfg interface{}) error {
	fields := flattenCfg(cfg, c.tag)
	errs := make(fieldErrors)

	for _, field := range fields {
		if err := c.processField(field); err != nil {
			errs[field.path()] = err
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// processField processes a single field and is called by processCfg
// for each field in cfg.
func (c *confucius) processField(field *field) error {
	if field.required && field.setDefault {
		return fmt.Errorf("field cannot have both a required validation and a default value")
	}

	if c.useEnv {
		if err := c.setFromEnv(field.v, field.path()); err != nil {
			return fmt.Errorf("unable to set from env: %v", err)
		}
	}

	if field.required && isZero(field.v) {
		return fmt.Errorf("required validation failed")
	}

	if field.setDefault && isZero(field.v) {
		if err := c.setDefaultValue(field.v, field.defaultVal); err != nil {
			return fmt.Errorf("unable to set default: %v", err)
		}
	}

	return nil
}

func (c *confucius) setFromEnv(fv reflect.Value, key string) error {
	key = c.formatEnvKey(key)
	if val, ok := os.LookupEnv(key); ok {
		return c.setValue(fv, val)
	}
	return nil
}

func (c *confucius) formatEnvKey(key string) string {
	// loggers[0].level --> loggers_0_level
	key = strings.NewReplacer(".", "_", "[", "_", "]", "").Replace(key)
	if c.envPrefix != "" {
		key = fmt.Sprintf("%s_%s", c.envPrefix, key)
	}
	return strings.ToUpper(key)
}

// setDefaultValue calls setValue but disallows booleans from
// being set.
func (c *confucius) setDefaultValue(fv reflect.Value, val string) error {
	if fv.Kind() == reflect.Bool {
		return fmt.Errorf("unsupported type: %v", fv.Kind())
	}
	return c.setValue(fv, val)
}

// setValue sets fv to val. it attempts to convert val to the correct
// type based on the field's kind. if conversion fails an error is
// returned.
// fv must be settable else this panics.
func (c *confucius) setValue(fv reflect.Value, val string) error {
	switch fv.Kind() {
	case reflect.Ptr:
		if fv.IsNil() {
			fv.Set(reflect.New(fv.Type().Elem()))
		}
		return c.setValue(fv.Elem(), val)
	case reflect.Slice:
		if err := c.setSlice(fv, val); err != nil {
			return err
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if _, ok := fv.Interface().(time.Duration); ok {
			d, err := time.ParseDuration(val)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(d))
		} else {
			i, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(f)
	case reflect.String:
		fv.SetString(val)
	case reflect.Struct: // struct is only allowed a default in the special case where it's a time.Time
		if _, ok := fv.Interface().(time.Time); ok {
			t, err := time.Parse(c.timeLayout, val)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(t))
		} else {
			return fmt.Errorf("unsupported type %s", fv.Kind())
		}
	default:
		return fmt.Errorf("unsupported type %s", fv.Kind())
	}
	return nil
}

// setSlice val to sv. val should be a Go slice formatted as a string
// (e.g. "[1,2]") and sv must be a slice value. if conversion of val
// to a slice fails then an error is returned.
// sv must be settable else this panics.
func (c *confucius) setSlice(sv reflect.Value, val string) error {
	ss := stringSlice(val)
	slice := reflect.MakeSlice(sv.Type(), len(ss), cap(ss))
	for i, s := range ss {
		if err := c.setValue(slice.Index(i), s); err != nil {
			return err
		}
	}
	sv.Set(slice)
	return nil
}
