package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
)

var opt options
var parser = flags.NewParser(&opt, flags.Default)

// getConfig reads and returns the configuration file
func getConfig() (*config, error) {
	if _, err := parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		default:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			return nil, fmt.Errorf("Error loading configuration file: %v", err)
		}
	}
	var cfg = config{}
	var koanf = koanf.New(".")
	if err := koanf.Load(file.Provider(opt.ConfigFile), toml.Parser()); err != nil {
		return nil, fmt.Errorf("Error loading configuration file: %v", err)
	}
	if err := koanf.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("Error loading configuration file: %v", err)
	}
	if opt.Debug {
		cfg.debug = true
	}

	return &cfg, nil
}
