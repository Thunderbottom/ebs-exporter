package main

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Injected during the build
var (
	buildDate    = "unknown"
	buildVersion = "unknown"
)

func getLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.InfoLevel)

	return logger
}

func main() {
	log := getLogger()
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	if config.debug {
		log.SetLevel(logrus.DebugLevel)
		log.Debug("Debug mode enabled")
	}

	hub := Hub{
		config: config,
		logger: log,
	}

	router := http.NewServeMux()
	router.Handle("/", injectContext(&hub, http.HandlerFunc(hub.defaultHandler)))
	router.Handle("/metrics", injectContext(&hub, http.HandlerFunc(hub.metricsHandler)))

	server := &http.Server{
		Addr:         config.Server.Address,
		Handler:      router,
		ReadTimeout:  config.Server.ReadTimeout * time.Millisecond,
		WriteTimeout: config.Server.WriteTimeout * time.Millisecond,
	}

	hub.logger.Infof("Starting server. Listening on: %v", config.Server.Address)
	if err := server.ListenAndServe(); err != nil {
		hub.logger.Fatalf("Error starting server: %v", err)
	}
}

func injectContext(app *Hub, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "app", app)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
