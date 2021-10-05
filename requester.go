package main

import (
	"context"
	"fmt"
	"sync"
	"time"

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

type Requester struct {
	h   host.Host
	dht *kaddht.IpfsDHT
	pm  *pb.ProtocolMessenger
	eh  *EventHub
}

func NewRequester(ctx context.Context, eh *EventHub) (*Requester, error) {
	// Generate a new identity
	key, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
	if err != nil {
		return nil, errors.Wrap(err, "generate key pair")
	}

	// Initialize a new libp2p host with the above identity
	var dht *kaddht.IpfsDHT
	h, err := libp2p.New(ctx,
		libp2p.Identity(key),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dht, err = kaddht.New(ctx, h)
			return dht, err
		}))
	if err != nil {
		return nil, errors.Wrap(err, "new libp2p host")
	}

	ms := &messageSenderImpl{
		host:      h,
		protocols: protocol.ConvertFromStrings([]string{"/ipfs/kad/1.0.0"}),
		strmap:    make(map[peer.ID]*peerMessageSender),
	}

	pm, err := pb.NewProtocolMessenger(ms)
	if err != nil {
		return nil, err
	}
	return &Requester{
		h:   h,
		dht: dht,
		pm:  pm,
		eh:  eh,
	}, nil
}

func (r *Requester) Bootstrap(ctx context.Context) error {
	for _, bp := range kaddht.GetDefaultBootstrapPeerAddrInfos() {
		log.WithField("type", "requester").Infoln("Connecting to bootstrap peer")
		if err := r.h.Connect(ctx, bp); err != nil {
			return errors.Wrap(err, "connecting to bootstrap peer")
		}
	}
	return nil
}

func (r *Requester) MonitorProviders(ctx context.Context, content *Content) error {
	logEntry := log.WithField("type", "requester")

	logEntry.Infoln("Getting closest peers")
	closest, err := r.dht.GetClosestPeers(ctx, string(content.cid.Hash()))
	if err != nil {
		return errors.Wrap(err, "get closest peers")
	}
	logEntry.Infof("Found %d peers", len(closest))

	logEntry.Infof("Starting monitoring")
	go func() {
		logEntry.Infoln("Querying closest peers for provider records")
		var wg sync.WaitGroup
		for _, c := range closest {
			log.WithField("targetID", c.Pretty()[:16]).Infoln("Mark as relevant")
			r.eh.MarkAsRelevant(c)
			wg.Add(1)
			go func(peerID peer.ID) {
				defer wg.Done()

				logEntry2 := logEntry.WithField("targetID", peerID.Pretty()[:16]).WithField("count", len(closest))

				for range time.Tick(time.Millisecond * 500) {
					logEntry2.Infoln("Getting providers...")

					r.eh.PushEvent(&MonitorProviderStart{
						BaseEvent: BaseEvent{
							ID:   peerID,
							Time: time.Now(),
						},
					})

					provs, _, err := r.pm.GetProviders(ctx, peerID, content.mhash)
					eevent := &MonitorProviderEnd{
						BaseEvent: BaseEvent{
							ID:   peerID,
							Time: time.Now(),
						},
					}
					if err != nil {
						logEntry2.WithError(err).Warnln("Could not get providers")
						eevent.Err = err
						r.eh.PushEvent(eevent)
						return
					} else if len(provs) > 0 {
						r.eh.PushEvent(eevent)
						logEntry2.Infoln("Found provider record!")
						return
					}

					eevent.Err = fmt.Errorf("not found")
					r.eh.PushEvent(eevent)
				}
			}(c)
		}
		wg.Wait()
		log.Infoln("All peers returned the provider record!")
	}()

	return nil
}
