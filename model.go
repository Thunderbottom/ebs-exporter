package main

import (
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/sirupsen/logrus"
)

// Exporter is a struct that contains an instance
// of AWS clients and job configuration
type Exporter struct {
	clients []MetricsCollector
	hub     *Hub
	job     *job
	logger  *logrus.Logger
	metrics *metrics.Set
	session *session.Session
}

// Hub contains the scraping configuration
// and an instance of the logger
type Hub struct {
	config *config
	logger *logrus.Logger
}

type awsCredentials struct {
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	Region    string `koanf:"region"`
	RoleARN   string `koanf:"role_arn"`
}

type config struct {
	debug  bool   `koanf:"debug"`
	Jobs   []job  `koanf:"jobs"`
	Server server `koanf:"server"`
}

type filter struct {
	Name  string `koanf:"name"`
	Value string `koanf:"value"`
}

type job struct {
	Name    string         `koanf:"name"`
	AWS     awsCredentials `koanf:"aws"`
	Filters []filter       `koanf:"filters"`
	Tags    []tag          `koanf:"tags"`
}

type server struct {
	Address      string        `koanf:"address"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
}

type tag struct {
	Tag         string `koanf:"tag"`
	ExportedTag string `koanf:"exported_tag"`
}

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Path to configuration file" default:"config.toml"`
	Debug      bool   `short:"d" long:"debug" description:"Enable debug level logging"`
}
