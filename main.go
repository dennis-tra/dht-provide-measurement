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

func main() {
	ctx := context.Background()

	// Generate random content that we'll provide in the DHT.
	content, err := NewRandomContent()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new random content"))
	}
	log.WithField("cid", content.cid.String()).Infof("Generated content")

	eh := NewEventHub()

	// Construct the requester (must come before new provider due to monkey patching)
	requester, err := NewRequester(ctx, eh)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new requester"))
	}

	// Construct the provider libp2p host
	provider, err := NewProvider(ctx, eh)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "new provider"))
	}

	// Bootstrap both libp2p hosts by connecting the canonical bootstrap peers.
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

	// Start pinging the closest peers to the random content from above for provider records.
	if err = requester.MonitorProviders(context.Background(), content); err != nil {
		log.Fatalln(errors.Wrap(err, "monitor provider"))
	}

	// Provide the random content from above.
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
