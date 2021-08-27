package http3

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"net"
	"strings"
	"sync"
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

type Sample struct {
	Time  time.Time
	Value uint64
}

type SampleQueue interface {
	Add(i int)
	Clear()
	Mean() float64
	String() string
}

type sampleQueue struct {
	mtx      sync.RWMutex
	queue    []Sample
	capacity int
}

func NewSampleQueue(capacity int) SampleQueue {
	return &sampleQueue{
		queue:    make([]Sample, 0, capacity),
		capacity: capacity,
	}
}

func (s *sampleQueue) Add(i int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	sample := Sample{
		Time:  time.Now(),
		Value: uint64(i),
	}

	if len(s.queue) < s.capacity {
		s.queue = append(s.queue, sample)
	} else {
		copy(s.queue[0:s.capacity-1], s.queue[1:])
		s.queue[s.capacity-1] = sample
	}
}

func (s *sampleQueue) Clear() {
	s.queue = make([]Sample, 0, s.capacity)
}

func (s *sampleQueue) Mean() float64 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	var mean uint64

	lenQueue := len(s.queue)
	if lenQueue < 1 {
		return 0
	}

	mean = 0
	for _, v := range s.queue {
		mean += v.Value
	}
	return float64(mean) / float64(lenQueue)
}

func (s *sampleQueue) String() string {
	return fmt.Sprintf("%f", s.Mean())
}

func (s *sampleQueue) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

type StatusEntry struct {
	InitialClientID quic.StatsClientID `json:"initial_client_id"`
	ClientID        quic.StatsClientID `json:"client_id"`
	Remote          net.Addr           `json:"remote_addr"`
	Session         quic.Session       `json:"-"`
	Status          Status             `json:"status"`
	Flows           int                `json:"flows"`
	LastRequest     string             `json:"last_request"`
	LastCwnd        SampleQueue        `json:"last_cwnd"`
	LastUpdate      time.Time          `json:"last_update"`
}

type HTTPStats interface {
	quic.ServerStats

	LastRequest(cID quic.StatsClientID, r string)

	All() []*StatusEntry
}

func NewStatusEntry(cID quic.StatsClientID, remote net.Addr, sess quic.Session, status Status) *StatusEntry {
	entry := &StatusEntry{InitialClientID: cID, ClientID: cID, Remote: remote, Session: sess, Status: status, LastUpdate: time.Now(), LastCwnd: NewSampleQueue(200)}
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
			"%s-%s - %s: updated: %s \t %s flows: %d, last req: %s, cwnd: %.2f MByte",
			string(s.InitialClientID),
			string(s.ClientID),
			s.Remote.String(),
			time.Now().Sub(s.LastUpdate).String(),
			s.Status.String(),
			s.Flows,
			s.LastRequest,
			float64(s.LastCwnd.Mean())/MByte,
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
