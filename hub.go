package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	pb "github.com/libp2p/go-libp2p-kad-dht/pb"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
)

type EventHub struct {
	mutex  sync.RWMutex
	events map[peer.ID][]Event
}

type Event interface {
	PeerID() peer.ID
}

func NewEventHub() *EventHub {
	return &EventHub{
		events: map[peer.ID][]Event{},
	}
}

func (eh *EventHub) RegisterForEvents(ctx context.Context, h host.Host) {
	var queryEvents <-chan *routing.QueryEvent
	ctx, queryEvents = routing.RegisterForQueryEvents(ctx)

	h.Network().Notify(eh)

	go eh.handleQueryEvents(queryEvents)
}

func (eh *EventHub) PushEvent(event Event) {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	events, found := eh.events[event.PeerID()]
	if !found {
		eh.events[event.PeerID()] = []Event{}
	}
	log.WithField("peerID", event.PeerID().Pretty()[:16]).
		WithField("event", fmt.Sprintf("%T", event)).
		Infoln("Pushing Event")

	eh.events[event.PeerID()] = append(events, event)
}

func (eh *EventHub) handleQueryEvents(queryEvents <-chan *routing.QueryEvent) {
	for event := range queryEvents {
		switch event.Type {
		case routing.SendingQuery:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Sending Query")
		case routing.PeerResponse:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Received Peer Response")
		case routing.QueryError:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Ecountered Query Error")
		case routing.DialingPeer:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Dialing Peer")
		default:
			panic(event.Type)
		}
	}
}

func (eh *EventHub) ensureSlice(peerID peer.ID) {
}

func (eh *EventHub) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) Connected(n network.Network, conn network.Conn) {
	eh.PushEvent(&ConnectedEvent{
		ID:   conn.RemotePeer(),
		Time: time.Now(),
	})
}

func (eh *EventHub) Disconnected(n network.Network, conn network.Conn) {
	eh.PushEvent(&DisconnectedEvent{
		ID:   conn.RemotePeer(),
		Time: time.Now(),
	})
}

func (eh *EventHub) OpenedStream(n network.Network, stream network.Stream) {
}

func (eh *EventHub) ClosedStream(n network.Network, stream network.Stream) {
}

type DialEvent struct {
	Transport string
	ID        peer.ID
	Maddr     ma.Multiaddr
	Error     error
	Start     time.Time
	End       time.Time
}

func (e *DialEvent) PeerID() peer.ID {
	return e.ID
}

type SendRequestEvent struct {
	ID       peer.ID
	Request  *pb.Message
	Response *pb.Message
	Error    error
	Start    time.Time
	End      time.Time
}

func (e *SendRequestEvent) PeerID() peer.ID {
	return e.ID
}

type SendMessageEvent struct {
	ID      peer.ID
	Message *pb.Message
	Error   error
	Start   time.Time
	End     time.Time
}

func (e *SendMessageEvent) PeerID() peer.ID {
	return e.ID
}

type ConnectedEvent struct {
	ID   peer.ID
	Time time.Time
}

func (e *ConnectedEvent) PeerID() peer.ID {
	return e.ID
}

type DisconnectedEvent struct {
	ID   peer.ID
	Time time.Time
}

func (e *DisconnectedEvent) PeerID() peer.ID {
	return e.ID
}
