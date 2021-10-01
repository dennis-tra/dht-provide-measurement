package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	kbucket "github.com/libp2p/go-libp2p-kbucket"

	"github.com/libp2p/go-libp2p-core/protocol"

	u "github.com/ipfs/go-ipfs-util"
	"go.uber.org/atomic"

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
	mutex     sync.RWMutex
	events    map[peer.ID][]Event
	relevant  sync.Map
	stopped   *atomic.Bool
	startTime time.Time
	stopTime  time.Time
}

type Event interface {
	PeerID() peer.ID
	TimeStamp() time.Time
	Error() error
}

func NewEventHub() *EventHub {
	return &EventHub{
		startTime: time.Now(),
		events:    map[peer.ID][]Event{},
		relevant:  sync.Map{},
		stopped:   atomic.NewBool(false),
	}
}

func (eh *EventHub) MarkAsRelevant(peerID peer.ID) {
	eh.relevant.Store(peerID, struct{}{})
}

func (eh *EventHub) RegisterForEvents(ctx context.Context, h host.Host) context.Context {
	var queryEvents <-chan *routing.QueryEvent
	ctx, queryEvents = routing.RegisterForQueryEvents(ctx)

	h.Network().Notify(eh)

	go eh.handleQueryEvents(queryEvents)

	return withProvideContext(ctx)
}

func (eh *EventHub) PushEvent(event Event) {
	if eh.stopped.Load() {
		return
	}

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

func (eh *EventHub) Serialize(content *Content) {
	f, err := os.Create("plot_data.csv")
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"peer_id",
		"distance",
		"time",
		"type",
		"error",
		"extra",
	})
	for peerID, events := range eh.events {
		distance := u.XOR(kbucket.ConvertPeerID(peerID), kbucket.ConvertKey(string(content.mhash)))

		for _, evt := range events {
			extra := ""
			switch event := evt.(type) {
			case *DialStart:
				extra = event.Maddr.String()
			case *DialEnd:
				extra = event.Maddr.String()
			case *OpenedStream:
				extra = protocol.ConvertToStrings([]protocol.ID{event.Protocol})[0]
			case *ClosedStream:
				extra = protocol.ConvertToStrings([]protocol.ID{event.Protocol})[0]
			case *SendRequestStart:
				extra = event.Request.Type.String()
			case *SendRequestEnd:
				if event.Err == nil {
					extra = event.Response.Type.String()
				}
			case *SendMessageStart:
				extra = event.Message.Type.String()
			case *DiscoveredPeer:
				extra = hex.EncodeToString(u.XOR(kbucket.ConvertPeerID(event.Discovered), kbucket.ConvertKey(string(content.mhash))))
			}
			w.Write([]string{
				peerID.Pretty(),
				hex.EncodeToString(distance),
				fmt.Sprintf("%.4f", evt.TimeStamp().Sub(eh.startTime).Seconds()),
				fmt.Sprintf("%T", evt),
				fmt.Sprintf("%t", evt.Error() != nil),
				extra,
			})
		}
	}
}

func (eh *EventHub) Stop(h host.Host) {
	eh.stopped.Store(true)
	eh.stopTime = time.Now()

	h.Network().StopNotify(eh)

	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	relevant := map[peer.ID][]Event{}
	eh.relevant.Range(func(key, value interface{}) bool {
		relevant[key.(peer.ID)] = eh.events[key.(peer.ID)]
		return true
	})
	eh.events = relevant
}

func (eh *EventHub) handleQueryEvents(queryEvents <-chan *routing.QueryEvent) {
	for event := range queryEvents {
		eh.MarkAsRelevant(event.ID)
		switch event.Type {
		case routing.SendingQuery:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Sending Query")
		case routing.PeerResponse:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Received Peer Response")
		case routing.QueryError:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Encountered Query Error")
		case routing.DialingPeer:
			log.WithField("peerID", event.ID.Pretty()[:16]).Infoln("Dialing Peer")
		default:
			panic(event.Type)
		}
	}
}

func (eh *EventHub) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) Connected(n network.Network, conn network.Conn) {
	eh.PushEvent(&ConnectedEvent{
		BaseEvent: BaseEvent{
			ID:   conn.RemotePeer(),
			Time: time.Now(),
		},
	})
}

func (eh *EventHub) Disconnected(n network.Network, conn network.Conn) {
	eh.PushEvent(&DisconnectedEvent{
		BaseEvent: BaseEvent{
			ID:   conn.RemotePeer(),
			Time: time.Now(),
		},
	})
}

func (eh *EventHub) OpenedStream(n network.Network, stream network.Stream) {
	eh.PushEvent(&OpenedStream{
		BaseEvent: BaseEvent{
			ID:   stream.Conn().RemotePeer(),
			Time: time.Now(),
		},
		Protocol: stream.Protocol(),
	})
}

func (eh *EventHub) ClosedStream(n network.Network, stream network.Stream) {
	eh.PushEvent(&ClosedStream{
		BaseEvent: BaseEvent{
			ID:   stream.Conn().RemotePeer(),
			Time: time.Now(),
		},
		Protocol: stream.Protocol(),
	})
}

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

type DialStart struct {
	BaseEvent
	Transport string
	Maddr     ma.Multiaddr
}

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
}

func (e *OpenStreamStart) Error() error {
	return nil
}

type OpenStreamEnd struct {
	BaseEvent
	Err error
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
