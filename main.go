package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cookielab/aws-instance-metadata-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	logLevel    = log.InfoLevel
	bindAddr    = flag.String("bind-addr", ":9189", "bind address for the metrics server")
	metricsPath = flag.String("metrics-path", "/metrics", "path to metrics endpoint")
	rawLevel    = flag.String("log-level", "info", "log level")
)

func init() {
	flag.Parse()

	parsedLevel, err := log.ParseLevel(*rawLevel)
	if err != nil {
		log.Fatal(err)
	}

	logLevel = parsedLevel
}

func main() {
	log.SetLevel(logLevel)
	log.Info("Starting aws-instance-metadata-exporter")

	log.Debug("registering term exporter")
	prometheus.MustRegister(collector.NewCollector())

	go serveMetrics()

	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	exitSignal := <-exitChannel
	log.WithFields(log.Fields{"signal": exitSignal}).Infof("Caught %s signal, exiting", exitSignal)
}

func serveMetrics() {
	log.Infof("Starting metric http endpoint on %s", *bindAddr)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(*bindAddr, nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html>
    <head><title>AWS Instance Metadata Exporter</title></head>
    <body>
    <h1>AWS Instance Metadata Exporter</h1>
    <p><a href="` + *metricsPath + `">Metrics</a></p>
    </body>
    </html>`))
}
