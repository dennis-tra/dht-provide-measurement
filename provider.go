package main

import (
	"bou.ke/monkey"
	"context"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pb "github.com/libp2p/go-libp2p-kad-dht/pb"
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

	ms := &messageSenderImpl{
		protocols: protocol.ConvertFromStrings([]string{"/ipfs/kad/1.0.0"}),
		strmap:    make(map[peer.ID]*peerMessageSender),
	}

	pm, err := pb.NewProtocolMessenger(ms)
	if err != nil {
		return nil, err
	}

	monkey.Patch(pb.NewProtocolMessenger, func(msgSender pb.MessageSender, opts ...pb.ProtocolMessengerOption) (*pb.ProtocolMessenger, error) {
		for _, o := range opts {
			if err := o(pm); err != nil {
				return nil, err
			}
		}
		return pm, nil
	})

	var dht *kaddht.IpfsDHT
	h, err := libp2p.New(ctx,
		libp2p.Identity(key),
		libp2p.Transport(NewTCPTransport),
		libp2p.Transport(NewWSTransport),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dht, err = kaddht.New(ctx, h)
			return dht, err
		}))
	if err != nil {
		return nil, errors.Wrap(err, "new libp2p host")
	}
	ms.host = h

	notifere := &network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			log.WithField("id", conn.RemotePeer().Pretty()[:16]).Infoln("Connected!")
		},
		OpenedStreamF: func(n network.Network, stream network.Stream) {
			log.WithField("id", stream.Conn().RemotePeer().Pretty()[:16]).Infoln("Opened stream")
		},
	}
	h.Network().Notify(notifere)
	return &Provider{
		h:   h,
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

var spanMap SpanMap

func (p *Provider) Provide(ctx context.Context, content *Content) error {
	logEntry := log.WithField("type", "provider")

	var span *trace.Span
	spanMap, span = InitSpanMap(ctx)
	defer span.End()

	ctx, events := routing.RegisterForQueryEvents(ctx)
	go func() {
		logEntry.Infoln("Registered for query")
		for event := range events {
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

	err := p.dht.Provide(ctx, content.contentID, true)

	spanMap.StopSpans()
	return err
}
