package main

import (
	"context"
	"github.com/libp2p/go-libp2p-core/peer"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"strings"
	"sync"
)

type SpanMap struct {
	rCtx context.Context // root context
	lk   sync.RWMutex
	m    map[peer.ID]SpanMapEntry
}

type SpanMapEntry struct {
	pCtx context.Context // parent context
	ctx  context.Context
	span *trace.Span
}

func InitSpanMap(ctx context.Context) (SpanMap, *trace.Span) {
	ctx, span := trace.StartSpan(ctx, "provide")
	return SpanMap{
		rCtx: ctx,
		lk:   sync.RWMutex{},
		m:    map[peer.ID]SpanMapEntry{},
	}, span
}

func (sm *SpanMap) SendQuery(id peer.ID) {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	var sCtx context.Context // span ctx
	sme, found := sm.m[id]
	if found {
		if sme.ctx != nil {
			sCtx = sme.ctx
		} else {
			sCtx = sme.pCtx
		}
		sme.span.End() // end a dial process
	} else {
		sCtx = sm.rCtx
		sme = SpanMapEntry{
			pCtx: sm.rCtx,
		}
	}
	log.Infoln("Send query")
	sme.ctx, sme.span = trace.StartSpan(sCtx, "sending_query")
	sme.span.AddAttributes(trace.StringAttribute("peerID", id.Pretty()[:16]))
	sm.m[id] = sme
}

func (sm *SpanMap) PeerResponse(id peer.ID, peers []*peer.AddrInfo) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	sme, found := sm.m[id]
	if !found {
		panic("peer response without query")
	}
	ids := []string{}

	for _, pi := range peers {
		ids = append(ids, pi.ID.String()[:16])
		if _, found := sm.m[pi.ID]; found {
			continue
		}
		nsme := SpanMapEntry{
			pCtx: sme.ctx,
		}
		nsme.ctx, nsme.span = trace.StartSpan(sme.ctx, "awaiting_use")
		nsme.span.AddAttributes(trace.StringAttribute("peerID", pi.ID.Pretty()[:16]))
		nsme.span.AddAttributes(trace.StringAttribute("responder", id.Pretty()[:16]))
		log.Infoln("Peer response fill map")
		sm.m[pi.ID] = nsme
	}
	sme.span.AddAttributes(trace.StringAttribute("responses", strings.Join(ids, ", ")))
	sme.span.End()
}

func (sm *SpanMap) DialingPeer(id peer.ID) {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	var sCtx context.Context // span ctx
	sme, found := sm.m[id]
	if found {
		if sme.ctx != nil {
			sCtx = sme.ctx
		} else {
			sCtx = sme.pCtx
		}
	} else {
		sCtx = sm.rCtx
		sme = SpanMapEntry{
			pCtx: sm.rCtx,
		}
	}
	log.Infoln("Dial Peer")
	sme.ctx, sme.span = trace.StartSpan(sCtx, "dialing_peer")
	sme.span.AddAttributes(trace.StringAttribute("peerID", id.Pretty()[:16]))
	sm.m[id] = sme
}

func (sm *SpanMap) QueryError(id peer.ID, extra string) {
	sm.lk.Lock()
	defer sm.lk.Unlock()
	sme, found := sm.m[id]
	if !found {
		panic("peer response without query")
	}
	sme.span.SetStatus(trace.Status{
		Code:    trace.StatusCodeInternal,
		Message: extra,
	})
	sme.span.End()
	log.Infoln("Send query")
	sm.m[id] = sme
}

func (sm *SpanMap) StopSpans() {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	for _, entry := range sm.m {
		if entry.span != nil {
			entry.span.SetStatus(
				trace.Status{
					Code:    trace.StatusCodeAborted,
					Message: "stopped after provide resolved",
				})
			entry.span.End()
		} else {
			log.Infoln("span nil")
		}
	}
}
