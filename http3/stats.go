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

	LastRequest(c protocol.ConnectionID, r string)
	LastCwnd(cId protocol.ConnectionID, c int)

	All() []StatusEntry
}

type dummyHTTPStats struct{}

func (d dummyHTTPStats) AddClient(cId protocol.ConnectionID, sess quic.Session) {}

func (d dummyHTTPStats) RetireClient(cId protocol.ConnectionID) {}

func (d dummyHTTPStats) AddFlow(cId protocol.ConnectionID) {}

func (d dummyHTTPStats) RemoveFlow(cId protocol.ConnectionID) {}

func (d dummyHTTPStats) NotifyChanged(oldID, newID protocol.ConnectionID) {}

func (d dummyHTTPStats) LastRequest(c protocol.ConnectionID, r string) {}

func (d dummyHTTPStats) LastCwnd(cId protocol.ConnectionID, c int) {}

func (d dummyHTTPStats) All() []StatusEntry { return nil }

func newDummyHTTPStats() HTTPStats {
	return &dummyHTTPStats{}
}
