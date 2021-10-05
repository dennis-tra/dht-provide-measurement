package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/routing"

	kbucket "github.com/libp2p/go-libp2p-kbucket"

	"github.com/libp2p/go-libp2p-core/protocol"

	u "github.com/ipfs/go-ipfs-util"
	"go.uber.org/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

type EventHub struct {
	mutex     sync.RWMutex
	events    map[peer.ID][]Event
	relevant  sync.Map
	stopped   *atomic.Bool
	startTime time.Time
	stopTime  time.Time
}

func NewEventHub() *EventHub {
	return &EventHub{
		events:   map[peer.ID][]Event{},
		relevant: sync.Map{},
		stopped:  atomic.NewBool(false),
	}
}

func (eh *EventHub) MarkAsRelevant(peerID peer.ID) {
	eh.relevant.Store(peerID, struct{}{})
}

func (eh *EventHub) PushEvent(event Event) {
	if eh == nil || eh.stopped.Load() {
		return
	}

	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	events, found := eh.events[event.PeerID()]
	if !found {
		eh.events[event.PeerID()] = []Event{}
	}
	// log.WithField("peerID", event.PeerID().Pretty()[:16]).
	//	WithField("event", fmt.Sprintf("%T", event)).
	//	Infoln("Pushing Event")

	eh.events[event.PeerID()] = append(events, event)
}

func (eh *EventHub) Start(ctx context.Context, h host.Host) context.Context {
	eh.startTime = time.Now()
	h.Network().Notify(eh)
	ctx, queryEvents := routing.RegisterForQueryEvents(ctx)
	go eh.handleQueryEvents(queryEvents)
	return ctx
}

func (eh *EventHub) handleQueryEvents(queryEvents <-chan *routing.QueryEvent) {
	for event := range queryEvents {
		eh.MarkAsRelevant(event.ID)
	}
}

func (eh *EventHub) Stop(h host.Host) {
	eh.stopped.Store(true)
	eh.stopTime = time.Now()

	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	h.Network().StopNotify(eh)

	// Since we collected all Dials and whatnot we filter the events
	// for peer IDs that were used for getting the closest peers in
	// the call to dht.Provide. Those relevant peers are marked as
	// relevant in the handleQueryEvents method.
	relevant := map[peer.ID][]Event{}
	eh.relevant.Range(func(key, value interface{}) bool {
		relevant[key.(peer.ID)] = eh.events[key.(peer.ID)]
		return true
	})
	eh.events = relevant
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

func (eh *EventHub) Serialize(content *Content) {
	f, err := os.Create("events.csv")
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Write header
	w.Write([]string{
		"peer_id",
		"distance",
		"time",
		"type",
		"has_error",
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
			case *OpenStreamStart:
				extra = strings.Join(protocol.ConvertToStrings(event.Protocols), ",")
			case *OpenStreamEnd:
				extra = strings.Join(protocol.ConvertToStrings(event.Protocols), ",")
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
			errorStr := ""
			if evt.Error() != nil {
				errorStr = strings.ReplaceAll(evt.Error().Error(), "\n", " ")
			}
			w.Write([]string{
				peerID.Pretty(),
				hex.EncodeToString(distance),
				fmt.Sprintf("%.6f", evt.TimeStamp().Sub(eh.startTime).Seconds()),
				fmt.Sprintf("%T", evt),
				fmt.Sprintf("%t", evt.Error() != nil),
				errorStr,
				extra,
			})
		}
	}
}
