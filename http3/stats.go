package http3

import (
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/internal/protocol"
)

type Status int

const (
	Alive Status = iota
	Inactive
	Retired
	Unknown
)

type StatusEntry struct {
	clientID    protocol.ConnectionID
	session     quic.Session
	status      Status
	lastRequest string
	lastCwnd    int
}

type HTTPStats interface {
	quic.ServerStats

	LastRequest(c quic.StatsClientID, r string)
	LastCwnd(cID quic.StatsClientID, c int)

	All() []StatusEntry
}

type dummyHTTPStats struct{}

func (d dummyHTTPStats) AddClient(_ quic.StatsClientID, _ quic.Session) {}

func (d dummyHTTPStats) RetireClient(_ quic.StatsClientID) {}

func (d dummyHTTPStats) AddFlow(_ quic.StatsClientID) {}

func (d dummyHTTPStats) RemoveFlow(_ quic.StatsClientID) {}

func (d dummyHTTPStats) NotifyChanged(_, _ quic.StatsClientID) {}

func (d dummyHTTPStats) LastRequest(_ quic.StatsClientID, _ string) {}

func (d dummyHTTPStats) LastCwnd(_ quic.StatsClientID, _ int) {}

func (d dummyHTTPStats) All() []StatusEntry { return nil }

func newDummyHTTPStats() HTTPStats {
	return &dummyHTTPStats{}
}
