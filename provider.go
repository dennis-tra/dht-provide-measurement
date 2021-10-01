package main

import (
	"context"
	"time"

	"bou.ke/monkey"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pb "github.com/libp2p/go-libp2p-kad-dht/pb"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Provider struct {
	h   host.Host
	dht *kaddht.IpfsDHT
	eh  *EventHub
}

func NewProvider(ctx context.Context) (*Provider, error) {
	eh := NewEventHub()

	key, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
	if err != nil {
		return nil, errors.Wrap(err, "generate key pair")
	}

	ms := &messageSenderImpl{
		protocols: protocol.ConvertFromStrings([]string{"/ipfs/kad/1.0.0"}),
		strmap:    make(map[peer.ID]*peerMessageSender),
		eventHub:  eh,
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
		libp2p.DefaultListenAddrs,
		libp2p.Transport(NewTCPTransport(eh)),
		libp2p.Transport(NewWSTransport(eh)),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dht, err = kaddht.New(ctx, h)
			return dht, err
		}))
	if err != nil {
		return nil, errors.Wrap(err, "new libp2p host")
	}
	ms.host = h

	return &Provider{
		h:   h,
		dht: dht,
		eh:  eh,
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
	ctx = p.eh.RegisterForEvents(ctx, p.h)
	p.eh.startTime = time.Now()
	err := p.dht.Provide(ctx, content.contentID, true)
	p.eh.Stop(p.h)
	p.eh.Serialize(content)
	return err
}
