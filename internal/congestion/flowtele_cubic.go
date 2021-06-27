package congestion

import (
	"fmt"
	"math"
	"time"

	"github.com/lucas-clemente/quic-go/internal/utils"

	"github.com/lucas-clemente/quic-go/internal/protocol"
)

type FlowTeleCubic struct {
	Cubic

	betaRaw                         float32 //nolint:structcheck,unused
	betaLastMaxRaw                  float32 //nolint:structcheck,unused
	lastMaxCongestionWindowAddDelta int64
	cwndAddDelta                    int64
	betaValue                       float32
	isThirdPhaseValue               bool
}

// NewFlowTeleCubic returns a new FlowTeleCubic instance
func NewFlowTeleCubic(clock Clock) *FlowTeleCubic {
	c := &FlowTeleCubic{
		Cubic: Cubic{
			clock:          clock,
			numConnections: defaultNumConnections,
		},
	}
	c.Reset()
	return c
}

// TODO:	Understand why the adjust functions are needed.
func (c *FlowTeleCubic) adjustBeta() {
	c.betaRaw = c.betaValue
	c.betaLastMaxRaw = 1 - (1-c.betaRaw)/2
}

// TODO:	Understand why the adjust functions are needed.
func (c *FlowTeleCubic) adjustLastMaxCongestionWindow() {
	c.lastMaxCongestionWindow = protocol.ByteCount(int64(c.lastMaxCongestionWindow) + c.lastMaxCongestionWindowAddDelta)
	c.lastMaxCongestionWindowAddDelta = 0
}

func (c *FlowTeleCubic) Reset() {
	c.epoch = time.Time{}
	c.lastMaxCongestionWindow = 0
	c.ackedBytesCount = 0
	c.estimatedTCPcongestionWindow = 0
	c.originPointCongestionWindow = 0
	c.timeToOriginPoint = 0
	c.lastTargetCongestionWindow = 0
}

func (c *FlowTeleCubic) alpha() float32 { //nolint:unused
	// TODO: 	Flowtele's original impl. uses a hardcoded beta here but then calls c.beta() in
	// 				CongestionWindowAfterPacketLoss. Which one is correct?
	// flowtele uses the hardcoded default beta value for the TCP fairness calculations
	b := float32(0.7)
	return 3 * float32(c.numConnections) * float32(c.numConnections) * (1 - b) / (1 + b)
}

func (c *FlowTeleCubic) beta() float32 { //nolint:unused
	fmt.Println("FLOWTELE BETA CALLED")
	return (float32(c.numConnections) - 1 + c.betaRaw) / float32(c.numConnections)
}

func (c *FlowTeleCubic) betaLastMax() float32 { //nolint:unused
	return (float32(c.numConnections) - 1 + c.betaLastMaxRaw) / float32(c.numConnections)
}

// OnApplicationLimited is called on ack arrival when sender is unable to use
// the available congestion window. Resets Cubic state during quiescence.
func (c *FlowTeleCubic) OnApplicationLimited() {
	// When sender is not using the available congestion window, the window does
	// not grow. But to be RTT-independent, Cubic assumes that the sender has been
	// using the entire window during the time since the beginning of the current
	// "epoch" (the end of the last loss recovery period). Since
	// application-limited periods break this assumption, we reset the epoch when
	// in such a period. This reset effectively freezes congestion window growth
	// through application-limited periods and allows Cubic growth to continue
	// when the entire window is being used.
	c.epoch = time.Time{}
}

// CongestionWindowAfterPacketLoss computes a new congestion window to use after
// a loss event. Returns the new congestion window in packets. The new
// congestion window is a multiplicative decrease of our current window.
func (c *FlowTeleCubic) CongestionWindowAfterPacketLoss(currentCongestionWindow protocol.ByteCount) protocol.ByteCount {
	c.adjustBeta()

	if currentCongestionWindow+maxDatagramSize < c.lastMaxCongestionWindow {
		// We never reached the old max, so assume we are competing with another
		// flow. Use our extra back off factor to allow the other flow to go up.
		c.lastMaxCongestionWindow = protocol.ByteCount(c.betaLastMax() * float32(currentCongestionWindow))
	} else {
		c.lastMaxCongestionWindow = currentCongestionWindow
	}
	c.epoch = time.Time{} // Reset time.

	c.adjustLastMaxCongestionWindow()

	return protocol.ByteCount(float32(currentCongestionWindow) * c.beta())
}

// CongestionWindowAfterAck computes a new congestion window to use after a received ACK.
// Returns the new congestion window in packets. The new congestion window
// follows a cubic function that depends on the time passed since last
// packet loss.
func (c *FlowTeleCubic) CongestionWindowAfterAck(ackedBytes protocol.ByteCount, currentCongestionWindow protocol.ByteCount, delayMin time.Duration, eventTime time.Time) protocol.ByteCount {
	c.ackedBytesCount += ackedBytes

	if c.epoch.IsZero() {
		// First ACK after a loss event.
		c.epoch = eventTime            // Start of epoch.
		c.ackedBytesCount = ackedBytes // Reset count.
		// Reset estimated_tcp_congestion_window_ to be in sync with cubic.
		c.estimatedTCPcongestionWindow = currentCongestionWindow
		if c.lastMaxCongestionWindow <= currentCongestionWindow {
			c.timeToOriginPoint = 0
			c.originPointCongestionWindow = currentCongestionWindow
		} else {
			c.timeToOriginPoint = uint32(math.Cbrt(float64(cubeFactor * (c.lastMaxCongestionWindow - currentCongestionWindow))))
			c.originPointCongestionWindow = c.lastMaxCongestionWindow
		}
	}

	// Change the time unit from microseconds to 2^10 fractions per second. Take
	// the round trip time in account. This is done to allow us to use shift as a
	// divide operator.
	elapsedTime := int64(eventTime.Add(delayMin).Sub(c.epoch)/time.Microsecond) << 10 / (1000 * 1000)

	// Right-shifts of negative, signed numbers have implementation-dependent
	// behavior, so force the offset to be positive, as is done in the kernel.
	offset := int64(c.timeToOriginPoint) - elapsedTime
	if offset < 0 {
		offset = -offset
	}

	deltaCongestionWindow := protocol.ByteCount(cubeCongestionWindowScale*offset*offset*offset) * maxDatagramSize >> cubeScale
	var targetCongestionWindow protocol.ByteCount
	if elapsedTime > int64(c.timeToOriginPoint) {
		targetCongestionWindow = c.originPointCongestionWindow + deltaCongestionWindow
	} else {
		targetCongestionWindow = c.originPointCongestionWindow - deltaCongestionWindow
	}
	// Limit the CWND increase to half the acked bytes.
	targetCongestionWindow = utils.MinByteCount(targetCongestionWindow, currentCongestionWindow+c.ackedBytesCount/2)

	// Increase the window by approximately Alpha * 1 MSS of bytes every
	// time we ack an estimated tcp window of bytes.  For small
	// congestion windows (less than 25), the formula below will
	// increase slightly slower than linearly per estimated tcp window
	// of bytes.
	c.estimatedTCPcongestionWindow += protocol.ByteCount(float32(c.ackedBytesCount) * c.alpha() * float32(maxDatagramSize) / float32(c.estimatedTCPcongestionWindow))
	c.ackedBytesCount = 0

	// We have a new cubic congestion window.
	c.lastTargetCongestionWindow = targetCongestionWindow

	// Compute target congestion_window based on cubic target and estimated TCP
	// congestion_window, use highest (fastest).
	if targetCongestionWindow < c.estimatedTCPcongestionWindow {
		targetCongestionWindow = c.estimatedTCPcongestionWindow
	}
	return targetCongestionWindow
}

func (c *FlowTeleCubic) SetNumConnections(n int) {
	c.Cubic.SetNumConnections(n)
}
