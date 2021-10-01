package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

func withProvideContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "provide-context", true)
}

func isProvideContext(ctx context.Context) bool {
	val := ctx.Value("provide-context")
	return val == true
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

	requester, err := NewRequester(ctx)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new requester"))
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return provider.Bootstrap(ctx)
	})
	group.Go(func() error {
		return requester.Bootstrap(ctx)
	})
	if err = group.Wait(); err != nil {
		log.Fatalln(errors.Wrap(err, "bootstrap err group"))
	}

	if err = requester.MonitorProviders(context.Background(), content, provider.eh); err != nil {
		log.Fatalln(errors.Wrap(err, "monitor provider"))
	}

	if err = provider.Provide(context.Background(), content); err != nil {
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
