package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/thunderbottom/ebs-exporter/exporters/ebs"
	"github.com/thunderbottom/ebs-exporter/pkg/exporter"
	"golang.org/x/sync/errgroup"
)

func initExporters(ex *exporter.Exporter, m *metrics.Set) {
	var (
		job = ex.Job()
		rc  *aws.Config
	)

	if job.AWS.RoleARN != "" {
		creds := stscreds.NewCredentials(ex.Session(), job.AWS.RoleARN)
		rc = &aws.Config{Credentials: creds}
	}

	ebsExp := ebs.New(ex.Job(), ex.Logger(), m, rc, ex.Session())
	ex.AddClient(ebsExp)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	var app = r.Context().Value("app").(*App)

	app.logger.Debugf("handling response for %s from %s", r.URL.Path, r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Welcome to ebs exporter. Visit /metrics."))
}

// metricsHandler handles the /metrics endpoint and executes all jobs
// specified in the config.toml for scraping metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var app = r.Context().Value("app").(*App)

	app.logger.Debugf("handling response for %s from %s", r.URL.Path, r.RemoteAddr)
	reqStart := time.Now()
	metricSet := metrics.NewSet()

	// Create a WaitGroup to run all the jobs concurrently
	wg := sync.WaitGroup{}
	wg.Add(len(app.config.Jobs))

	for _, job := range app.config.Jobs {
		job := job
		app.logger.Debugf("starting job: %s", job.Name)
		go func() {
			defer wg.Done()
			var g errgroup.Group

			ex := exporter.New(app.logger, &job, metricSet)
			initExporters(ex, metricSet)

			for _, client := range ex.Clients() {
				g.Go(client.Collect)
			}

			// Stop scraping metrics for the entire job
			// if something fails. Sets scrape status for
			// the job to 0
			var status float64 = 1
			if err := g.Wait(); err != nil {
				app.logger.Error("stopping job: %s", job.Name)
				status = 0
			}

			metricSet.GetOrCreateGauge(fmt.Sprintf(`ebs_exporter_up{job="%s"}`, job.Name), func() float64 {
				return status
			})
		}()
	}
	wg.Wait()

	metricSet.WritePrometheus(w)
	app.logger.Debugf("done! total time taken for request: %s", time.Since(reqStart))
}
