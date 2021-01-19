package config

import (
	"fmt"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
)

type awsCredentials struct {
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	Region    string `koanf:"region"`
	RoleARN   string `koanf:"role_arn"`
}

type Config struct {
	Debug  bool   `koanf:"debug"`
	Jobs   []Job  `koanf:"jobs"`
	Server server `koanf:"server"`
}

type Filter struct {
	Name  string `koanf:"name"`
	Value string `koanf:"value"`
}

type Job struct {
	Name    string         `koanf:"name"`
	AWS     awsCredentials `koanf:"aws"`
	Filters []Filter       `koanf:"filters"`
	Tags    []Tag          `koanf:"tags"`
}

type server struct {
	Address      string        `koanf:"address"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
}

type Tag struct {
	Tag         string `koanf:"tag"`
	ExportedTag string `koanf:"exported_tag"`
}

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Path to configuration file" default:"config.toml"`
	Debug      bool   `short:"d" long:"debug" description:"Enable debug level logging"`
}

var (
	opt    options
	parser = flags.NewParser(&opt, flags.Default)
)

// ReadConfig reads and returns the configuration file
func ReadConfig() (*Config, error) {
	if _, err := parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		default:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			return nil, fmt.Errorf("error loading configuration file: %s", err)
		}
	}

	var cfg = Config{}
	var koanf = koanf.New(".")
	if err := koanf.Load(file.Provider(opt.ConfigFile), toml.Parser()); err != nil {
		return nil, fmt.Errorf("error loading configuration file: %s", err)
	}
	if err := koanf.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("error loading configuration file: %s", err)
	}

	if opt.Debug {
		cfg.Debug = true
	}

	return &cfg, nil
}
