package main

import (
	"context"
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
	"sync"
	"time"
)

type Requester struct {
	h   host.Host
	dht *kaddht.IpfsDHT
	pm  *pb.ProtocolMessenger
}

func NewRequester(ctx context.Context) (*Requester, error) {
	key, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
	if err != nil {
		return nil, errors.Wrap(err, "generate key pair")
	}

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

	ms := &msgSender{
		h:         h,
		protocols: protocol.ConvertFromStrings([]string{"/ipfs/kad/1.0.0"}),
		timeout:   time.Minute,
	}

	pm, err := pb.NewProtocolMessenger(ms)
	if err != nil {
		return nil, err
	}
	return &Requester{
		h:   h,
		dht: dht,
		pm:  pm,
	}, nil
}

func (p *Requester) Bootstrap(ctx context.Context) error {
	for _, bp := range kaddht.GetDefaultBootstrapPeerAddrInfos() {
		log.WithField("type", "requester").Infoln("Connecting to bootstrap peer")
		if err := p.h.Connect(ctx, bp); err != nil {
			return errors.Wrap(err, "connecting to bootstrap peer")
		}
	}
	return nil
}

func (p *Requester) MonitorProviders(ctx context.Context, content *Content) error {
	logEntry := log.WithField("type", "requester")
	logEntry.Infoln("Getting closest peers")
	closest, err := p.dht.GetClosestPeers(ctx, string(content.contentID.Hash()))
	if err != nil {
		return errors.Wrap(err, "get closest peers")
	}
	logEntry.Infof("Found %d peers", len(closest))

	// TODO: Are we actually connected to the closest peers?
	//var wg sync.WaitGroup
	//for _, c := range closest {
	//	wg.Add(1)
	//	go func(c peer.ID) {
	//		defer wg.Done()
	//		logEntry = logEntry.WithField("targetID", c.Pretty()[:16])
	//		logEntry.Infoln("Establish connection to peer...")
	//		var pi peer.AddrInfo
	//		if pi = p.dht.FindLocal(c); pi.ID != "" {
	//			logEntry.Infoln("Already connected!")
	//			return
	//		}
	//		logEntry.Infoln("Looking for multi addresses")
	//		pi, err = p.dht.FindPeer(ctx, c)
	//		if err != nil {
	//			logEntry.WithError(err).Warnln("Could not find peer")
	//			return
	//		}
	//		logEntry.Infoln("Connecting to peer")
	//		if err = p.h.Connect(ctx, pi); err != nil {
	//			logEntry.WithError(err).Warnln("Could not connect to peer")
	//			return
	//		}
	//	}(c)
	//}
	//wg.Wait()

	logEntry.Infof("Starting monitoring")
	go func() {
		logEntry.Infoln("Querying closest peers for provider records")
		var wg sync.WaitGroup
		for _, c := range closest {
			wg.Add(1)
			go func(peerID peer.ID) {
				defer wg.Done()

				logEntry2 := logEntry.WithField("targetID", peerID.Pretty()[:16])
				for range time.Tick(time.Second) {
					logEntry2.Infoln("Getting providers...")
					provs, _, err := p.pm.GetProviders(ctx, peerID, content.mhash)
					if err != nil {
						logEntry2.WithError(err).Warnln("Could not get providers")
						return
					}
					if len(provs) > 0 {
						logEntry2.Infoln("Found provider record!")
						return
					}
				}
			}(c)
		}
		wg.Wait()
		log.Infoln("All peers returned the provider record!")
	}()

	return nil
}
