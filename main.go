package main

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/thunderbottom/ebs-exporter/pkg/config"
)

// App contains the scraping configuration
// and an instance of the logger.
type App struct {
	config *config.Config
	logger *logrus.Logger
}

// Injected during the build.
var (
	buildDate    = "unknown"
	buildVersion = "unknown"
)

// getLogger returns an instance of logrus.
func getLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.InfoLevel)

	return logger
}

// injectContext is a wrapper over HTTP handlers that injects the "app" context.
func injectContext(app *App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "app", app)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newApp returns an instance of App.
func newApp(log *logrus.Logger) (*App, error) {
	cfg, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Debug {
		log.SetLevel(logrus.DebugLevel)
		log.Debug("debug mode enabled")
	}

	app := &App{
		config: cfg,
		logger: log,
	}

	return app, nil
}

func main() {
	log := getLogger()
	app, err := newApp(log)
	if err != nil {
		log.Fatalf("an error occurred while initializing app: %s", err)
	}

	router := http.NewServeMux()
	router.Handle("/", injectContext(app, http.HandlerFunc(defaultHandler)))
	router.Handle("/metrics", injectContext(app, http.HandlerFunc(metricsHandler)))

	server := &http.Server{
		Addr:         app.config.Server.Address,
		Handler:      router,
		ReadTimeout:  app.config.Server.ReadTimeout * time.Millisecond,
		WriteTimeout: app.config.Server.WriteTimeout * time.Millisecond,
	}

	app.logger.Infof("starting server. listening on: %s", app.config.Server.Address)
	if err := server.ListenAndServe(); err != nil {
		app.logger.Fatalf("error starting server: %s", err)
	}
}
