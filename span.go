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
	span *trace.Span
	lk   sync.RWMutex
	m    map[peer.ID]SpanMapEntry
}

type SpanMapEntry struct {
	ctx    context.Context
	span   *trace.Span
	status string
}

func InitSpanMap(ctx context.Context) (SpanMap, *trace.Span) {
	ctx, span := trace.StartSpan(ctx, "provide")
	return SpanMap{
		rCtx: ctx,
		span: span,
		lk:   sync.RWMutex{},
		m:    map[peer.ID]SpanMapEntry{},
	}, span
}

func (sm *SpanMap) SendQuery(id peer.ID) {
	log.Infoln("Send query")
	sm.lk.Lock()
	defer sm.lk.Unlock()

	sme, found := sm.m[id]
	if found {
		sme.span.End()
	} else {
		sme = SpanMapEntry{
			ctx:  sm.rCtx,
			span: sm.span,
		}
	}
	sme.ctx, sme.span = trace.StartSpan(sme.ctx, "sending_query")
	sme.status = "sending_query"
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
		if f, found := sm.m[pi.ID]; found {
			sme.span.AddLink(trace.Link{
				TraceID: f.span.SpanContext().TraceID,
				SpanID:  f.span.SpanContext().SpanID,
				Type:    trace.LinkTypeChild,
				Attributes: map[string]interface{}{
					"reason": "peer replied same closer peer",
				},
			})
			continue
		}
		nsme := SpanMapEntry{}
		nsme.ctx, nsme.span = trace.StartSpan(sme.ctx, "awaiting_use")
		nsme.status = "awaiting_use"
		nsme.span.AddAttributes(trace.StringAttribute("peerID", pi.ID.Pretty()[:16]))
		nsme.span.AddAttributes(trace.StringAttribute("responder", id.Pretty()[:16]))
		log.Infoln("Peer response fill map")
		sm.m[pi.ID] = nsme
	}
	sme.span.AddAttributes(trace.StringAttribute("responses", strings.Join(ids, ", ")))
	sme.span.End()
}

func (sm *SpanMap) DialingPeer(id peer.ID) {
	log.Infoln("Dial Peer")
	sm.lk.Lock()
	defer sm.lk.Unlock()

	sme, found := sm.m[id]
	if found {
		sme.span.End()
	} else {
		sme = SpanMapEntry{
			ctx:  sm.rCtx,
			span: sm.span,
		}
	}
	sme.ctx, sme.span = trace.StartSpan(sme.ctx, "dialing_peer")
	sme.status = "dialing_peer"
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

	if extra == "context canceled" {
		sme.span.SetStatus(trace.Status{
			Code:    trace.StatusCodeCancelled,
			Message: extra,
		})
	} else {
		sme.span.SetStatus(trace.Status{
			Code:    trace.StatusCodeInternal,
			Message: extra,
		})
	}
	sme.span.End()
	log.Infoln("Send query")
	sm.m[id] = sme
}

func (sm *SpanMap) StopSpans() {
	sm.lk.Lock()
	defer sm.lk.Unlock()

	for _, entry := range sm.m {
		if entry.status == "awaiting_use" {
			continue
		}
		entry.span.SetStatus(
			trace.Status{
				Code:    trace.StatusCodeAborted,
				Message: "stopped after provide resolved",
			})
		entry.span.End()
	}
}
