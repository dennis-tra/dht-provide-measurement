package main

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-tcp-transport"
	websocket "github.com/libp2p/go-ws-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type TCPTransport struct {
	transport *tcp.TcpTransport
}

func NewTCPTransport(upgrader *tptu.Upgrader) *TCPTransport {
	return &TCPTransport{
		transport: tcp.NewTCPTransport(upgrader),
	}
}
func (t *TCPTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	dial, err := t.transport.Dial(ctx, raddr, p)
	fmt.Println("Dialed with err", err)
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
	transport *websocket.WebsocketTransport
}

func NewWSTransport(upgrader *tptu.Upgrader) *TCPTransport {
	return &TCPTransport{
		transport: tcp.NewTCPTransport(upgrader),
	}
}
func (ws *WSTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	dial, err := ws.transport.Dial(ctx, raddr, p)
	fmt.Println("WS dialed with err", err)
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
