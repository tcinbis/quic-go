package http3

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"net"
	"strings"
	"time"
)

type Status int

func (s Status) String() string {
	switch s {
	case Alive:
		return "Alive"
	case Inactive:
		return "Inactive"
	case Retired:
		return "Retired"
	case Unknown:
		fallthrough
	default:
		return "Unkown"
	}
}

const (
	Alive Status = iota
	Inactive
	Retired
	Unknown
)

const (
	Bit   = 1
	Byte  = 8 * Bit
	KByte = 1000 * Byte
	MByte = 1000 * KByte
)

type StatusEntry struct {
	ClientID    quic.StatsClientID `json:"client_id"`
	Remote      net.Addr           `json:"remote_addr"`
	Session     quic.Session       `json:"-"`
	Status      Status             `json:"status"`
	Flows       int                `json:"flows"`
	LastRequest string             `json:"last_request"`
	LastCwnd    int                `json:"last_cwnd"`
	LastUpdate  time.Time          `json:"last_update"`
}

type HTTPStats interface {
	quic.ServerStats

	LastRequest(cID quic.StatsClientID, r string)

	All() []*StatusEntry
}

func NewStatusEntry(cID quic.StatsClientID, remote net.Addr, sess quic.Session, status Status) *StatusEntry {
	entry := &StatusEntry{ClientID: cID, Remote: remote, Session: sess, Status: status, LastUpdate: time.Now()}
	if remote == nil {
		entry.Remote = &net.UDPAddr{}
		go checkForRemoteAvailable(sess, entry)
	}
	return entry
}

func checkForRemoteAvailable(sess quic.Session, e *StatusEntry) {
	for {
		if rAddr := sess.RemoteAddr(); rAddr != nil {
			e.Remote = rAddr
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (s *StatusEntry) String() string {
	var sb strings.Builder
	sb.WriteString(
		fmt.Sprintf(
			"%s - %s: updated: %s \t %s flows: %d, last req: %s, cwnd: %.2f MByte",
			string(s.ClientID),
			s.Remote.String(),
			time.Now().Sub(s.LastUpdate).String(),
			s.Status.String(),
			s.Flows,
			s.LastRequest,
			float64(s.LastCwnd)/MByte,
		),
	)

	return sb.String()
}

func (s *StatusEntry) Updated() {
	s.LastUpdate = time.Now()
}

type dummyHTTPStats struct{}

func (d dummyHTTPStats) AddClient(_ quic.StatsClientID, _ quic.Session) {}

func (d dummyHTTPStats) RetireClient(_ quic.StatsClientID) {}

func (d dummyHTTPStats) AddFlow(_ quic.StatsClientID) {}

func (d dummyHTTPStats) RemoveFlow(_ quic.StatsClientID) {}

func (d dummyHTTPStats) NotifyChanged(_, _ quic.StatsClientID) {}

func (d dummyHTTPStats) LastRequest(_ quic.StatsClientID, _ string) {}

func (d dummyHTTPStats) LastCwnd(_ quic.StatsClientID, _ int) {}

func (d dummyHTTPStats) All() []*StatusEntry { return nil }

func newDummyHTTPStats() HTTPStats {
	return &dummyHTTPStats{}
}
