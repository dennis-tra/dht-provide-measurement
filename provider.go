package main

import (
	"context"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

func (p *Provider) Bootstrap(ctx context.Context) error{
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

	ctx, events := routing.RegisterForQueryEvents(ctx)
	go func() {
		logEntry.Infoln("Registered for query")
		for event := range events {
			logEntry = logEntry.WithField("peer", event.ID.Pretty()[:16])
			switch event.Type {
			case routing.SendingQuery:
				logEntry.Infoln("Query Event: SendingQuery")
			case routing.PeerResponse:
				logEntry.Infoln("Query Event: PeerResponse")
			case routing.FinalPeer:
				logEntry.Infoln("Query Event: FinalPeer")
			case routing.QueryError:
				logEntry.Infoln("Query Event: QueryError")
			case routing.Provider:
				logEntry.Infoln("Query Event: Provider")
			case routing.Value:
				logEntry.Infoln("Query Event: Value")
			case routing.AddingPeer:
				logEntry.Infoln("Query Event: AddingPeer")
			case routing.DialingPeer:
				logEntry.Infoln("Query Event: DialingPeer")
			}
		}
		logEntry.Infoln("Registered for query done")
	}()
	logEntry.Infoln("Providing content")
	return p.dht.Provide(ctx, content.contentID, true)
}
