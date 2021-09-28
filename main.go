package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	//if err := ipfslog.SetLogLevel("dht/RtRefreshManager", "DEBUG"); err != nil {
	//	log.Fatalln(errors.Wrap(err, "set log level"))
	//}
}

type Content struct {
	raw       []byte
	mhash     mh.Multihash
	contentID cid.Cid
}

func NewRandomContent() (*Content, error) {
	raw := make([]byte, 1024)
	_, err := rand.Read(raw)
	if err != nil {
		return nil, errors.Wrap(err, "read rand data")
	}
	hash := sha256.New()
	hash.Write(raw)

	mhash, err := mh.Encode(hash.Sum(nil), mh.SHA2_256)
	if err != nil {
		return nil, errors.Wrap(err, "encode multi hash")
	}

	return &Content{
		raw:       raw,
		mhash:     mhash,
		contentID: cid.NewCidV0(mhash),
	}, nil
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

	// provider.InitRoutingTable() // <- takes a long time

	if err = requester.MonitorProviders(context.Background(), content); err != nil {
		log.Fatalln(errors.Wrap(err, "monitor provider"))
	}

	if err = provider.Provide(context.Background(), content); err != nil {
		log.Fatalln(errors.Wrap(err, "provide"))
	}
	log.Infoln("Provided record!")

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
