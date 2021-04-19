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
	cubicSend cubicSender

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
	cuSender := newCubicSender(clock, rttStats, reno, initialCongestionWindow, maxCongestionWindow, tracer)
	cuSender.cubic = NewFlowTeleCubic(clock)
	return &flowTeleCubicSender{
		cubicSend:               *cuSender,
		flowTeleSignalInterface: flowTeleSignal,
	}
}

func (c *flowTeleCubicSender) slowStartThresholdUpdated() {
	// todo(cyrill) do we need the actual packet received time or is time.Now() sufficient?
	c.flowTeleSignalInterface.PacketsLost(time.Now(), uint64(c.cubicSend.slowStartThreshold))
}

func (c *flowTeleCubicSender) ApplyControl(beta float64, cwnd_adjust int64, cwnd_max_adjust int64, use_conservative_allocation bool) bool { //nolint:stylecheck
	fmt.Printf("FLOWTELE CC: ApplyControl(%f, %d, %d, %t)\n", beta, cwnd_adjust, cwnd_max_adjust, use_conservative_allocation)
	flowTeleCubic := c.checkFlowTeleCubicAlgorithm()

	flowTeleCubic.lastMaxCongestionWindowAddDelta = cwnd_max_adjust
	flowTeleCubic.cwndAddDelta = cwnd_adjust
	flowTeleCubic.betaValue = float32(beta)
	flowTeleCubic.isThirdPhaseValue = use_conservative_allocation
	return true
}

func (c *flowTeleCubicSender) SetFixedRate(rateInBitsPerSecond Bandwidth) {
	fmt.Printf("FLOWTELE CC: SetFixedRate(%d)\n", rateInBitsPerSecond)
	c.useFixedBandwidth = true
	c.fixedBandwidth = rateInBitsPerSecond
}

func (c *flowTeleCubicSender) TimeUntilSend(bytesInFlight protocol.ByteCount) time.Time {
	return c.cubicSend.TimeUntilSend(bytesInFlight)
}

func (c *flowTeleCubicSender) HasPacingBudget() bool {
	return c.cubicSend.HasPacingBudget()
}

func (c *flowTeleCubicSender) OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	c.cubicSend.OnPacketSent(sentTime, bytesInFlight, packetNumber, bytes, isRetransmittable)
}

func (c *flowTeleCubicSender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return c.cubicSend.CanSend(bytesInFlight)
}

func (c *flowTeleCubicSender) MaybeExitSlowStart() {
	c.cubicSend.MaybeExitSlowStart()
}

func (c *flowTeleCubicSender) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time) {
	c.cubicSend.OnPacketAcked(number, ackedBytes, priorInFlight, eventTime)
	c.flowTeleSignalInterface.PacketsAcked(time.Now(), uint64(c.cubicSend.GetCongestionWindow()), uint64(priorInFlight), uint64(ackedBytes))
}

func (c *flowTeleCubicSender) OnPacketLost(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	c.cubicSend.OnPacketLost(number, lostBytes, priorInFlight)
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	c.cubicSend.OnRetransmissionTimeout(packetsRetransmitted)
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) OnConnectionMigration() {
	c.cubicSend.OnConnectionMigration()
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) InSlowStart() bool {
	return c.cubicSend.InSlowStart()
}

func (c *flowTeleCubicSender) InRecovery() bool {
	return c.cubicSend.InRecovery()
}

func (c *flowTeleCubicSender) GetCongestionWindow() protocol.ByteCount {
	if c.useFixedBandwidth {
		cwnd := BandwidthFromDelta(protocol.ByteCount(c.fixedBandwidth), c.cubicSend.rttStats.LatestRTT())
		fmt.Printf("Setting CWND Window to %d\n", cwnd)
		return protocol.ByteCount(cwnd)
	}
	return c.cubicSend.GetCongestionWindow()
}

func (c *flowTeleCubicSender) checkFlowTeleCubicAlgorithm() *FlowTeleCubic {
	f, ok := c.cubicSend.cubic.(*FlowTeleCubic)
	if !ok {
		panic("Received non-flowtele cubic sender.")
	}
	return f
}
