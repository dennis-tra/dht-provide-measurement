package main

import (
	"crypto/rand"
	"crypto/sha256"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
)

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
