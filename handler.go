package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sync/errgroup"
)

func (hub *Hub) defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	hub.logger.Debugf("Handling response for %s from %s", r.URL.Path, r.RemoteAddr)
	w.Write([]byte("Welcome to ebs exporter. Visit /metrics."))
}

// metricsHandler handles the /metrics endpoint and executes all jobs
// specified in the config.toml for scraping metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var (
		app = r.Context().Value("app").(*Hub)
	)

	app.logger.Debugf("Handling response for %s from %s", r.URL.Path, r.RemoteAddr)
	reqStart := time.Now()
	metricSet := metrics.NewSet()

	// Create a WaitGroup to run all the jobs concurrently
	wg := sync.WaitGroup{}
	wg.Add(len(app.config.Jobs))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, job := range app.config.Jobs {
		job := job
		app.logger.Debugf("Starting Job: %v", job.Name)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
			}

			exporter := app.NewExporter(&job, metricSet)
			var g errgroup.Group
			for _, client := range exporter.clients {
				g.Go(client.Collect)
			}
			// Stop scraping metrics for the entire job
			// if something fails. Sets scrape status for
			// the job to 0
			var status float64 = 1
			if err := g.Wait(); err != nil {
				cancel()
				app.logger.Error("Stopping Job: %v", job.Name)
				status = 0
			}
			metricSet.GetOrCreateGauge(fmt.Sprintf(`ebs_exporter_up{job="%s"}`, job.Name), func() float64 {
				return status
			})
		}()
	}
	wg.Wait()

	metricSet.WritePrometheus(w)
	app.logger.Debugf("Done! Total time taken for request: %v", time.Since(reqStart))
}
