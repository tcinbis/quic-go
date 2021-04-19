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
	cuSender := newCubicSender(clock, rttStats, reno, initialCongestionWindow, maxCongestionWindow, tracer)
	cuSender.cubic = NewFlowTeleCubic(clock)
	return &flowTeleCubicSender{
		cubicSender:             *cuSender,
		flowTeleSignalInterface: flowTeleSignal,
	}
}

func (c *flowTeleCubicSender) slowStartThresholdUpdated() {
	// todo(cyrill) do we need the actual packet received time or is time.Now() sufficient?
	c.flowTeleSignalInterface.PacketsLost(time.Now(), uint64(c.cubicSender.slowStartThreshold))
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
	return c.cubicSender.TimeUntilSend(bytesInFlight)
}

func (c *flowTeleCubicSender) HasPacingBudget() bool {
	return c.cubicSender.HasPacingBudget()
}

func (c *flowTeleCubicSender) OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool) {
	c.cubicSender.OnPacketSent(sentTime, bytesInFlight, packetNumber, bytes, isRetransmittable)
}

func (c *flowTeleCubicSender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return c.cubicSender.CanSend(bytesInFlight)
}

func (c *flowTeleCubicSender) MaybeExitSlowStart() {
	c.cubicSender.MaybeExitSlowStart()
}

func (c *flowTeleCubicSender) OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time) {
	c.cubicSender.OnPacketAcked(number, ackedBytes, priorInFlight, eventTime)
	c.flowTeleSignalInterface.PacketsAcked(time.Now(), uint64(c.cubicSender.GetCongestionWindow()), uint64(priorInFlight), uint64(ackedBytes))
}

func (c *flowTeleCubicSender) OnPacketLost(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
	c.cubicSender.OnPacketLost(number, lostBytes, priorInFlight)
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	c.cubicSender.OnRetransmissionTimeout(packetsRetransmitted)
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) OnConnectionMigration() {
	c.cubicSender.OnConnectionMigration()
	c.slowStartThresholdUpdated()
}

func (c *flowTeleCubicSender) checkFlowTeleCubicAlgorithm() *FlowTeleCubic {
	f, ok := c.cubicSender.cubic.(*FlowTeleCubic)
	if !ok {
		panic("Received non-flowtele cubic sender.")
	}
	return f
}
