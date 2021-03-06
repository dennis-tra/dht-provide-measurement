package main

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pb "github.com/libp2p/go-libp2p-kad-dht/pb"
	ma "github.com/multiformats/go-multiaddr"
)

// The Event interface must be implemented by all events
// that need to be tracked.
type Event interface {
	PeerID() peer.ID
	TimeStamp() time.Time
	Error() error
}

// BaseEvent captures the minimum number of fields and event
// needs to have. This is intended to be embedded into other
// structs.
type BaseEvent struct {
	ID   peer.ID
	Time time.Time
}

func (e *BaseEvent) PeerID() peer.ID {
	return e.ID
}

func (e *BaseEvent) TimeStamp() time.Time {
	return e.Time
}

func (e *BaseEvent) Error() error {
	return nil
}

// The DialStart event is dispatch when the TCP or Websocket
// transport modules start dialing a certain peer under the
// given multi address.
type DialStart struct {
	BaseEvent
	Transport string
	Maddr     ma.Multiaddr
}

// The DialEnd event is dispatch when the TCP or Websocket
// transport modules have finished dialing a certain peer under
// the given multi address.
type DialEnd struct {
	BaseEvent
	Transport string
	Maddr     ma.Multiaddr
	Err       error
}

func (e *DialEnd) Error() error {
	return e.Err
}

type SendRequestStart struct {
	BaseEvent
	Request *pb.Message
}

type SendRequestEnd struct {
	BaseEvent
	Response *pb.Message
	Err      error
}

func (e *SendRequestEnd) Error() error {
	return e.Err
}

type SendMessageStart struct {
	BaseEvent
	Message *pb.Message
}

type SendMessageEnd struct {
	BaseEvent
	Err error
}

func (e *SendMessageEnd) Error() error {
	return e.Err
}

type OpenStreamStart struct {
	BaseEvent
	Protocols []protocol.ID
}

func (e *OpenStreamStart) Error() error {
	return nil
}

type OpenStreamEnd struct {
	BaseEvent
	Err       error
	Protocols []protocol.ID
}

func (e *OpenStreamEnd) Error() error {
	return e.Err
}

type ConnectedEvent struct {
	BaseEvent
}

type DisconnectedEvent struct {
	BaseEvent
}

type OpenedStream struct {
	BaseEvent
	Protocol protocol.ID
}

type ClosedStream struct {
	BaseEvent
	Protocol protocol.ID
}

type DiscoveredPeer struct {
	BaseEvent
	Discovered peer.ID
}

type MonitorProviderStart struct {
	BaseEvent
}

type MonitorProviderEnd struct {
	BaseEvent
	Err error
}

func (e *MonitorProviderEnd) Error() error {
	return e.Err
}
