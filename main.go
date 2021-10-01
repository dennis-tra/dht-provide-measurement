package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

type contextKey struct{}

func withProvideContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, "provide")
}

func isProvideContext(ctx context.Context) bool {
	val := ctx.Value(contextKey{})
	return val == "provide"
}

func main() {
	ctx := context.Background()

	content, err := NewRandomContent()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new random content"))
	}
	log.WithField("cid", content.contentID.String()).Infof("Generated content")

	provider, err := NewProvider(ctx)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new provider"))
	}

	if err = provider.Bootstrap(ctx); err != nil {
		log.Fatalln(errors.Wrap(err, "bootstrap provider"))
	}

	// To identify all relevant calls we mark this context
	if err = provider.Provide(withProvideContext(ctx), content); err != nil {
		log.Fatalln(errors.Wrap(err, "provide"))
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Infoln(sig)
		done <- true
	}()

	log.Infoln("Awaiting user signal")
	<-done
	log.Infoln("Exiting")
}

func initZipkin() {
	zipkinep, err := openzipkin.NewEndpoint("dht", "localhost:5454")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "open zipkin endpoint"))
	}
	reporter := zipkinHTTP.NewReporter("http://localhost:9411/api/v2/spans")
	trace.RegisterExporter(zipkin.NewExporter(reporter, zipkinep))
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
}
