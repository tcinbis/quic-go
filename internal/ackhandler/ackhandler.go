package ackhandler

import (
	"github.com/lucas-clemente/quic-go/flowtele"
	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/utils"
	"github.com/lucas-clemente/quic-go/logging"
)

// NewAckHandler creates a new SentPacketHandler and a new ReceivedPacketHandler
func NewAckHandler(
	initialPacketNumber protocol.PacketNumber,
	initialMaxDatagramSize protocol.ByteCount,
	rttStats *utils.RTTStats,
	pers protocol.Perspective,
	tracer logging.ConnectionTracer,
	logger utils.Logger,
	version protocol.VersionNumber,
) (SentPacketHandler, ReceivedPacketHandler) {
	sph := newSentPacketHandler(initialPacketNumber, initialMaxDatagramSize, rttStats, pers, tracer, logger)
	return sph, newReceivedPacketHandler(sph, rttStats, logger, version)
}

// NewFlowTeleAckHandler creates a new FlowTeleSentPacketHandler and a new ReceivedPacketHandler
func NewFlowTeleAckHandler(
	initialPacketNumber protocol.PacketNumber,
	rttStats *utils.RTTStats,
	pers protocol.Perspective,
	traceCallback func(quictrace.Event),
	tracer logging.ConnectionTracer,
	logger utils.Logger,
	version protocol.VersionNumber,
	signal *flowtele.FlowTeleSignal,
) (FlowTeleSentPacketHandler, ReceivedPacketHandler) {
	sph := newFlowTeleSentPacketHandler(initialPacketNumber, rttStats, pers, traceCallback, tracer, logger, signal)
	return sph, newReceivedPacketHandler(sph, rttStats, logger, version)
}
