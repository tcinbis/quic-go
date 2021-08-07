package congestion

import (
	"math"
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"
)

// Bandwidth of a connection
type Bandwidth uint64

const infBandwidth Bandwidth = math.MaxUint64

const (
	// BitsPerSecond is 1 bit per second
	BitsPerSecond Bandwidth = 1
	// BytesPerSecond is 1 byte per second
	BytesPerSecond = 8 * BitsPerSecond
)

// BandwidthFromDelta calculates the bandwidth from a number of bytes and a time delta
// Unit being returned is Bits. We multiply by time.Second because time.Duration is measured in Nanoseconds by default.
func BandwidthFromDelta(bytes protocol.ByteCount, delta time.Duration) Bandwidth {
	return Bandwidth(bytes) * Bandwidth(time.Second) / Bandwidth(delta) * BytesPerSecond
}
