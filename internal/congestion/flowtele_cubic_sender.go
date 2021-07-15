package congestion

import (
	"fmt"
	"time"

	"github.com/lucas-clemente/quic-go/flowtele"
	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/utils"
	"github.com/lucas-clemente/quic-go/logging"
)

// flowTeleCubicSender works as a thin wrapper around cubicSender but can intercept any exported method.
type flowTeleCubicSender struct {
	cubicSender

	flowTeleSignalInterface *flowtele.FlowTeleSignal
	useFixedBandwidth       bool
	fixedBandwidth          Bandwidth
}

// Test whether we can assign flowTeleCubicSender to the FlowTeleSendAlgorithm interface
var (
	_ FlowTeleSendAlgorithm               = &flowTeleCubicSender{}
	_ FlowteleSendAlgorithmWithDebugInfos = &flowTeleCubicSender{}
)

func NewFlowTeleCubicSender(clock Clock, rttStats *utils.RTTStats, reno bool, tracer logging.ConnectionTracer, flowTeleSignal *flowtele.FlowTeleSignal) *flowTeleCubicSender {
	c := &flowTeleCubicSender{
		cubicSender: cubicSender{
			rttStats:                   rttStats,
			largestSentPacketNumber:    protocol.InvalidPacketNumber,
			largestAckedPacketNumber:   protocol.InvalidPacketNumber,
			largestSentAtLastCutback:   protocol.InvalidPacketNumber,
			initialCongestionWindow:    initialCongestionWindow,
			initialMaxCongestionWindow: maxCongestionWindow,
			congestionWindow:           initialCongestionWindow,
			minCongestionWindow:        minCongestionWindow,
			slowStartThreshold:         maxCongestionWindow,
			maxCongestionWindow:        maxCongestionWindow,
			cubic:                      NewFlowTeleCubic(clock),
			clock:                      clock,
			reno:                       reno,
			tracer:                     tracer,
		},
		flowTeleSignalInterface: flowTeleSignal,
	}
	c.pacer = newPacer(c.BandwidthEstimate)
	if c.tracer != nil {
		c.lastState = logging.CongestionStateSlowStart
		c.tracer.UpdatedCongestionState(logging.CongestionStateSlowStart)
	}
	return c
}
func (c *flowTeleCubicSender) adjustCongestionWindow() {
	if c.useFixedBandwidth {
		srtt := c.rttStats.SmoothedRTT()
		// If we haven't measured an rtt, we cannot estimate the cwnd
		if srtt != 0 {
			c.congestionWindow = utils.MinByteCount(c.maxCongestionWindow, protocol.ByteCount(math.Ceil(float64(DeltaBytesFromBandwidth(c.fixedBandwidth, srtt)))))
			fmt.Printf("FLOWTELE CC: set congestion window to %d (%d), fixed bw = %d, srtt = %v\n", c.GetCongestionWindow(), c.congestionWindow, c.fixedBandwidth, srtt)
		}
	} else if c.cubic.cwndAddDelta != 0 {
			fmt.Printf("FLOWTELE CC: add cwndAddDelta %d to congestion window %d\n", c.cubic.cwndAddDelta, c.congestionWindow)
		c.congestionWindow = utils.MaxByteCount(
			c.minCongestionWindow,
			utils.MinByteCount(
				c.maxCongestionWindow,
				protocol.ByteCount(int64(c.congestionWindow)+c.cubic.cwndAddDelta)))
		c.cubic.cwndAddDelta = 0
	}

	if c.cubic.isThirdPhaseValue {
		c.cubic.CongestionWindowAfterPacketLoss(c.congestionWindow)
		c.cubic.isThirdPhaseValue = false
	}
}

func (c *flowTeleCubicSender) slowStartThresholdUpdated() {
	// todo(cyrill) do we need the actual packet received time or is time.Now() sufficient?
	go c.flowTeleSignalInterface.PacketsLost(time.Now(), uint64(c.cubicSender.slowStartThreshold))
}

func (c *flowTeleCubicSender) ApplyControl(beta float64, cwnd_adjust int64, cwnd_max_adjust int64, use_conservative_allocation bool) bool { //nolint:stylecheck
	fmt.Printf("FLOWTELE CC: ApplyControl(%f, %d, %d, %t) mod\n", beta, cwnd_adjust, cwnd_max_adjust, use_conservative_allocation)
	flowTeleCubic := c.checkFlowTeleCubicAlgorithm()

	flowTeleCubic.lastMaxCongestionWindowAddDelta = cwnd_max_adjust
	flowTeleCubic.cwndAddDelta = cwnd_adjust
	flowTeleCubic.betaValue = float32(beta)
	flowTeleCubic.isThirdPhaseValue = use_conservative_allocation
	return true
}

func (c *flowTeleCubicSender) SetFixedRate(rateInBitPerSecond Bandwidth) {
	fmt.Printf("FLOWTELE CC: SetFixedRate(%d)\n", rateInBitPerSecond)
	c.useFixedBandwidth = true
	c.fixedBandwidth = rateInBitPerSecond
}

func (c *flowTeleCubicSender) TimeUntilSend(_ protocol.ByteCount) time.Time {
	return c.pacer.TimeUntilSend()
}

func (c *flowTeleCubicSender) HasPacingBudget() bool {
	return c.pacer.Budget(c.clock.Now()) >= maxDatagramSize
}

func (c *flowTeleCubicSender) OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	c.pacer.SentPacket(sentTime, bytes)
	if !isRetransmittable {
		return
	}
	c.largestSentPacketNumber = packetNumber
	c.hybridSlowStart.OnPacketSent(packetNumber)
}

func (c *flowTeleCubicSender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return bytesInFlight < c.GetCongestionWindow()
}

func (c *flowTeleCubicSender) InRecovery() bool {
	return c.largestAckedPacketNumber != protocol.InvalidPacketNumber && c.largestAckedPacketNumber <= c.largestSentAtLastCutback
}

func (c *flowTeleCubicSender) InSlowStart() bool {
	return c.GetCongestionWindow() < c.slowStartThreshold
}

func (c *flowTeleCubicSender) GetCongestionWindow() protocol.ByteCount {
	cwnd := c.congestionWindow
	//if c.useFixedBandwidth {
	//	//fmt.Printf("RTT in sec: %f or %s ", float64(c.rttStats.LatestRTT().Seconds()), c.rttStats.LatestRTT().String())
	//	cwnd = protocol.ByteCount(math.Max((float64(c.fixedBandwidth) * float64(c.rttStats.SmoothedRTT().Seconds())), 1))
	//}
	return cwnd
}

func (c *flowTeleCubicSender) MaybeExitSlowStart() {
	if c.InSlowStart() && c.hybridSlowStart.ShouldExitSlowStart(c.rttStats.LatestRTT(), c.rttStats.MinRTT(), c.GetCongestionWindow()/maxDatagramSize) {
		// exit slow start
		c.slowStartThreshold = c.congestionWindow
		c.maybeTraceStateChange(logging.CongestionStateCongestionAvoidance)
	}
}

func (c *flowTeleCubicSender) OnPacketAcked(ackedPacketNumber protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time) {
	c.largestAckedPacketNumber = utils.MaxPacketNumber(ackedPacketNumber, c.largestAckedPacketNumber)
	if c.InRecovery() {
		return
	}
	c.maybeIncreaseCwnd(ackedPacketNumber, ackedBytes, priorInFlight, eventTime)
	if c.InSlowStart() {
		c.hybridSlowStart.OnPacketAcked(ackedPacketNumber)
	}
	go c.flowTeleSignalInterface.PacketsAcked(time.Now(), uint64(c.GetCongestionWindow()), uint64(priorInFlight), uint64(ackedBytes))
}

func (c *flowTeleCubicSender) OnPacketLost(packetNumber protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	// TCP NewReno (RFC6582) says that once a loss occurs, any losses in packets
	// already sent should be treated as a single loss event, since it's expected.
	if packetNumber <= c.largestSentAtLastCutback {
		return
	}
	c.lastCutbackExitedSlowstart = c.InSlowStart()
	c.maybeTraceStateChange(logging.CongestionStateRecovery)

	if c.reno {
		c.congestionWindow = protocol.ByteCount(float64(c.congestionWindow) * renoBeta)
	} else {
		c.congestionWindow = c.cubic.CongestionWindowAfterPacketLoss(c.congestionWindow)
	}
	if c.congestionWindow < c.minCongestionWindow {
		c.congestionWindow = c.minCongestionWindow
	}
	c.slowStartThreshold = c.congestionWindow
	c.largestSentAtLastCutback = c.largestSentPacketNumber
	// reset packet count from congestion avoidance mode. We start
	// counting again when we're out of recovery.
	c.numAckedPackets = 0
	// Call to flowtele signal method is handled in slowStartThresholdUpdated
	c.slowStartThresholdUpdated()
}

// Called when we receive an ack. Normal TCP tracks how many packets one ack
// represents, but quic has a separate ack for each packet.
func (c *flowTeleCubicSender) maybeIncreaseCwnd(
	_ protocol.PacketNumber,
	ackedBytes protocol.ByteCount,
	priorInFlight protocol.ByteCount,
	eventTime time.Time,
) {
	// Do not increase the congestion window unless the sender is close to using
	// the current window.
	if !c.isCwndLimited(priorInFlight) {
		c.cubic.OnApplicationLimited()
		c.maybeTraceStateChange(logging.CongestionStateApplicationLimited)
		c.adjustCongestionWindow()
		return
	}
	if c.congestionWindow >= c.maxCongestionWindow {
		return
	}
	if c.InSlowStart() {
		// TCP slow start, exponential growth, increase by one for each ACK.
		c.congestionWindow += maxDatagramSize
		c.maybeTraceStateChange(logging.CongestionStateSlowStart)
		return
	}
	// Congestion avoidance
	c.maybeTraceStateChange(logging.CongestionStateCongestionAvoidance)
	if c.reno {
		// Classic Reno congestion avoidance.
		c.numAckedPackets++
		if c.numAckedPackets >= uint64(c.congestionWindow/maxDatagramSize) {
			c.congestionWindow += maxDatagramSize
			c.numAckedPackets = 0
		}
	} else {
		c.congestionWindow = utils.MinByteCount(c.maxCongestionWindow, c.cubic.CongestionWindowAfterAck(ackedBytes, c.congestionWindow, c.rttStats.MinRTT(), eventTime))
		c.adjustCongestionWindow()
	}
}

func (c *flowTeleCubicSender) isCwndLimited(bytesInFlight protocol.ByteCount) bool {
	congestionWindow := c.GetCongestionWindow()
	if bytesInFlight >= congestionWindow {
		return true
	}
	availableBytes := congestionWindow - bytesInFlight
	slowStartLimited := c.InSlowStart() && bytesInFlight > congestionWindow/2
	return slowStartLimited || availableBytes <= maxBurstBytes
}

// BandwidthEstimate returns the current bandwidth estimate
func (c *flowTeleCubicSender) BandwidthEstimate() Bandwidth {
	srtt := c.rttStats.SmoothedRTT()
	if srtt == 0 {
		// If we haven't measured an rtt, the bandwidth estimate is unknown.
		return infBandwidth
	}

	if c.useFixedBandwidth {
		// if a fixed bandwidth is set return it
		return c.fixedBandwidth * BitsPerSecond
	}

	return BandwidthFromDelta(c.GetCongestionWindow(), srtt)
}

func (c *flowTeleCubicSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	c.largestSentAtLastCutback = protocol.InvalidPacketNumber
	if !packetsRetransmitted {
		return
	}
	c.hybridSlowStart.Restart()
	c.cubic.Reset()
	c.slowStartThreshold = c.congestionWindow / 2
	c.congestionWindow = c.minCongestionWindow
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) OnConnectionMigration() {
	c.hybridSlowStart.Restart()
	c.largestSentPacketNumber = protocol.InvalidPacketNumber
	c.largestAckedPacketNumber = protocol.InvalidPacketNumber
	c.largestSentAtLastCutback = protocol.InvalidPacketNumber
	c.lastCutbackExitedSlowstart = false
	c.cubic.Reset()
	c.numAckedPackets = 0
	c.congestionWindow = c.initialCongestionWindow
	c.slowStartThreshold = c.initialMaxCongestionWindow
	c.maxCongestionWindow = c.initialMaxCongestionWindow
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) maybeTraceStateChange(new logging.CongestionState) {
	if c.tracer == nil || new == c.lastState {
		return
	}
	c.tracer.UpdatedCongestionState(new)
	c.lastState = new
}

func (c *flowTeleCubicSender) checkFlowTeleCubicAlgorithm() *FlowTeleCubic {
	f, ok := c.cubicSender.cubic.(*FlowTeleCubic)
	if !ok {
		panic("Received non-flowtele cubic sender.")
	}
	return f
}
