package congestion

import (
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"
)

// A SendAlgorithm performs congestion control
type SendAlgorithm interface {
	TimeUntilSend(bytesInFlight protocol.ByteCount) time.Time
	HasPacingBudget() bool
	OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool)
	CanSend(bytesInFlight protocol.ByteCount) bool
	MaybeExitSlowStart()
	OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time)
	OnPacketLost(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount)
	OnRetransmissionTimeout(packetsRetransmitted bool)
	OnConnectionMigration()
	SetMaxDatagramSize(protocol.ByteCount)
	BandwidthEstimate() Bandwidth
}

// A SendAlgorithmWithDebugInfos is a SendAlgorithm that exposes some debug infos
type SendAlgorithmWithDebugInfos interface {
	SendAlgorithm
	InSlowStart() bool
	InRecovery() bool
	GetCongestionWindow() protocol.ByteCount
}

type CubicInterface interface {
	Reset()
	OnApplicationLimited()
	CongestionWindowAfterPacketLoss(currentCongestionWindow protocol.ByteCount) protocol.ByteCount
	CongestionWindowAfterAck(ackedBytes protocol.ByteCount, currentCongestionWindow protocol.ByteCount, delayMin time.Duration, eventTime time.Time) protocol.ByteCount
	SetNumConnections(n int)
}

type FlowTeleCongestionControlModifier interface {
	ApplyControl(beta float64, cwnd_adjust int64, cwnd_max_adjust int64, use_conservative_allocation bool) bool //nolint:stylecheck
	SetFixedRate(rateInBitPerSecond Bandwidth)
}

type FlowTeleSendAlgorithm interface {
	SendAlgorithm
	FlowTeleCongestionControlModifier
}

type FlowteleSendAlgorithmWithDebugInfos interface {
	SendAlgorithmWithDebugInfos
	FlowTeleCongestionControlModifier
}
