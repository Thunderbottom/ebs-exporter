package main

import (
	"github.com/VictoriaMetrics/metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
)

// MetricsCollector is an interface for
// a set of methods to interact with AWS
type MetricsCollector interface {
	Collect() error
}

// NewExporter returns a new instance of Exporter for a job config
func (hub *Hub) NewExporter(j *job, m *metrics.Set) *Exporter {
	hub.logger.Debugf("Setting up exporter for job: %v", j.Name)
	exporter := &Exporter{
		job:     j,
		logger:  hub.logger,
		metrics: m,
	}
	config := &aws.Config{
		Region: aws.String(j.AWS.Region),
	}
	if j.AWS.AccessKey != "" && j.AWS.SecretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(
			j.AWS.AccessKey,
			j.AWS.SecretKey,
			"")
	}
	exporter.session = session.Must(session.NewSessionWithOptions(session.Options{
		Config: *config,
	}))
	var roleConfig *aws.Config
	if j.AWS.RoleARN != "" {
		hub.logger.Debugf("Assuming Role: %v", j.AWS.RoleARN)
		creds := stscreds.NewCredentials(exporter.session, j.AWS.RoleARN)
		roleConfig = &aws.Config{Credentials: creds}
	}
	exporter.initClients(roleConfig)
	return exporter
}

// initClients initializes all the clients for an exporter
func (ex *Exporter) initClients(roleConfig *aws.Config) {
	ex.clients = append(ex.clients, ex.newEC2Client(roleConfig))
}
