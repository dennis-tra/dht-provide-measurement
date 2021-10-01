package main

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multiaddr"
	"go.opencensus.io/trace"
	"sync"
)

type EventType string

const (
	AwaitingUse  EventType = "awaiting_use"
	SendingQuery EventType = "sending_query"
	PeerResponse EventType = "peer_response"
	QueryError   EventType = "query_error"
	DialingPeer  EventType = "dialing_peer"
	Connected    EventType = "connected"
	Disconnected EventType = "disconnected"
)

func (et EventType) String() string {
	return string(et)
}

type EventHub struct {
	mutex   sync.RWMutex
	states  map[peer.ID]*PeerState
	rootCtx context.Context
}

type PeerState struct {
	ctx       context.Context
	span      *trace.Span
	lastEvent EventType
	events    []EventType
	peerCtx   context.Context
}

func InitEventHub(ctx context.Context, h host.Host) (context.Context, *EventHub) {
	var queryEvents <-chan *routing.QueryEvent
	ctx, queryEvents = routing.RegisterForQueryEvents(ctx)

	eh := &EventHub{
		rootCtx: ctx,
		states:  map[peer.ID]*PeerState{},
	}
	h.Network().Notify(eh)

	go eh.handleQueryEvents(queryEvents)

	return ctx, eh
}

func (eh *EventHub) AddEvent(peerID peer.ID, eventType EventType) {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	eh.ensureSlice(peerID)

	state := eh.currentState(peerID)

	switch state.lastEvent {
	case AwaitingUse:
		switch eventType {
		case DialingPeer:
			state.span.End()
			state.ctx, state.span = trace.StartSpan(state.ctx, DialingPeer.String())
		}
	}
}

func (eh *EventHub) handleQueryEvents(queryEvents <-chan *routing.QueryEvent) {
	for event := range queryEvents {
		switch event.Type {
		case routing.SendingQuery:
			eh.AddEvent(event.ID, SendingQuery)
		case routing.PeerResponse:
			eh.AddEvent(event.ID, PeerResponse)
		case routing.QueryError:
			eh.AddEvent(event.ID, QueryError)
		case routing.DialingPeer:
			eh.AddEvent(event.ID, DialingPeer)
		default:
			panic(event.Type)
		}
	}
}

func (eh *EventHub) currentState(peerID peer.ID) *PeerState {
	state, found := eh.states[peerID]
	if !found {
		ctx, span := trace.StartSpan(eh.rootCtx, AwaitingUse.String())
		span.AddAttributes(trace.StringAttribute("peerID", peerID.Pretty()[:16]))
		state = &PeerState{
			peerCtx:   ctx,
			ctx:       ctx,
			span:      span,
			lastEvent: AwaitingUse,
			events:    []EventType{AwaitingUse},
		}
		eh.states[peerID] = state
	}
	return state
}

func (eh *EventHub) ensureSlice(peerID peer.ID) {
}

func (eh *EventHub) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
}

func (eh *EventHub) Connected(n network.Network, conn network.Conn) {
	eh.AddEvent(conn.RemotePeer(), Connected)
}

func (eh *EventHub) Disconnected(n network.Network, conn network.Conn) {
	eh.AddEvent(conn.RemotePeer(), Disconnected)
}

func (eh *EventHub) OpenedStream(n network.Network, stream network.Stream) {
}

func (eh *EventHub) ClosedStream(n network.Network, stream network.Stream) {
}
