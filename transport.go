package main

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-tcp-transport"
	websocket "github.com/libp2p/go-ws-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type TCPTransport struct {
	eventHub  *EventHub
	transport *tcp.TcpTransport
}

func NewTCPTransport(eh *EventHub) func(upgrader *tptu.Upgrader) *TCPTransport {
	return func(upgrader *tptu.Upgrader) *TCPTransport {
		return &TCPTransport{
			eventHub:  eh,
			transport: tcp.NewTCPTransport(upgrader),
		}
	}
}

func (t *TCPTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	start := time.Now()
	dial, err := t.transport.Dial(ctx, raddr, p)
	if isProvideContext(ctx) {
		event := &DialEvent{
			Transport: "tcp",
			ID:        p,
			Maddr:     raddr,
			Error:     err,
			Start:     start,
			End:       time.Now(),
		}
		t.eventHub.PushEvent(event)
	}
	return dial, err
}

func (t *TCPTransport) CanDial(addr ma.Multiaddr) bool {
	return t.transport.CanDial(addr)
}

func (t *TCPTransport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	return t.transport.Listen(laddr)
}

func (t *TCPTransport) Protocols() []int {
	return t.transport.Protocols()
}

func (t *TCPTransport) Proxy() bool {
	return t.transport.Proxy()
}

type WSTransport struct {
	eventHub  *EventHub
	transport *websocket.WebsocketTransport
}

func NewWSTransport(eh *EventHub) func(upgrader *tptu.Upgrader) *WSTransport {
	return func(upgrader *tptu.Upgrader) *WSTransport {
		return &WSTransport{
			eventHub:  eh,
			transport: websocket.New(upgrader),
		}
	}
}

func (ws *WSTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	start := time.Now()
	dial, err := ws.transport.Dial(ctx, raddr, p)
	if isProvideContext(ctx) {
		event := &DialEvent{
			Transport: "ws",
			ID:        p,
			Maddr:     raddr,
			Error:     err,
			Start:     start,
			End:       time.Now(),
		}
		ws.eventHub.PushEvent(event)
	}
	return dial, err
}

func (ws *WSTransport) CanDial(addr ma.Multiaddr) bool {
	return ws.transport.CanDial(addr)
}

func (ws *WSTransport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	return ws.transport.Listen(laddr)
}

func (ws *WSTransport) Protocols() []int {
	return ws.transport.Protocols()
}

func (ws *WSTransport) Proxy() bool {
	return ws.transport.Proxy()
}
