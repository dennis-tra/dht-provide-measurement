package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/zipkin"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

type Provider struct {
	h   host.Host
	dht *kaddht.IpfsDHT
}

func NewProvider(ctx context.Context) (*Provider, error) {
	key, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
	if err != nil {
		return nil, errors.Wrap(err, "generate key pair")
	}

	var dht *kaddht.IpfsDHT
	node, err := libp2p.New(ctx,
		libp2p.Identity(key),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dht, err = kaddht.New(ctx, h)
			return dht, err
		}))
	if err != nil {
		return nil, errors.Wrap(err, "new libp2p host")
	}

	return &Provider{
		h:   node,
		dht: dht,
	}, nil
}

func (p *Provider) Bootstrap(ctx context.Context) error {
	for _, bp := range kaddht.GetDefaultBootstrapPeerAddrInfos() {
		log.WithField("type", "provider").Infoln("Connecting to bootstrap peer")
		if err := p.h.Connect(ctx, bp); err != nil {
			return errors.Wrap(err, "connecting to bootstrap peer")
		}
	}
	return nil
}

func (p *Provider) InitRoutingTable() {
	<-p.dht.RefreshRoutingTable()
}

func (p *Provider) Provide(ctx context.Context, content *Content) error {
	logEntry := log.WithField("type", "provider")

	// Trace exporter: Zipkin
	localEndpoint, err := openzipkin.NewEndpoint("dht_provide_measurement", "localhost:5454")
	if err != nil {
		log.Fatalf("Failed to create the local zipkinEndpoint: %v", err)
	}
	reporter := zipkinHTTP.NewReporter("http://localhost:9411/api/v2/spans")
	ze := zipkin.NewExporter(reporter, localEndpoint)
	trace.RegisterExporter(ze)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	spanMap, span := InitSpanMap(ctx)
	defer span.End()

	ctx, events := routing.RegisterForQueryEvents(ctx)
	go func() {
		logEntry.Infoln("Registered for query")
		for event := range events {
			logEntry = logEntry.WithField("peer", event.ID.Pretty()[:16])
			switch event.Type {
			case routing.SendingQuery:
				spanMap.SendQuery(event.ID)
			case routing.PeerResponse:
				spanMap.PeerResponse(event.ID, event.Responses)
			case routing.QueryError:
				spanMap.QueryError(event.ID, event.Extra)
			case routing.DialingPeer:
				spanMap.DialingPeer(event.ID)
			default:
				panic(event.Type)
			}
		}
		logEntry.Infoln("Registered for query done")
	}()
	logEntry.Infoln("Providing content")
	err = p.dht.Provide(ctx, content.contentID, true)

	spanMap.StopSpans()
	return err
}
