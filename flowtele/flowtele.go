package flowtele

import (
	"time"
)

func CreateFlowteleSignalInterface(newSrttMeasurement func(t time.Time, srtt time.Duration), packetsLost func(t time.Time, newSlowStartThreshold uint64), packetsLostRatio func(t time.Time, lostRatio float64), packetsAcked func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64)) *FlowTeleSignal {
	if newSrttMeasurement == nil {
		newSrttMeasurement = func(t time.Time, srtt time.Duration) {}
	}

	if packetsLost == nil {
		packetsLost = func(t time.Time, newSlowStartThreshold uint64) {}
	}

	if packetsLostRatio == nil {
		packetsLostRatio = func(t time.Time, lostRatio float64) {}
	}

	if packetsAcked == nil {
		packetsAcked = func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64) {}
	}

	return &FlowTeleSignal{NewSrttMeasurement: newSrttMeasurement, PacketsLost: packetsLost, PacketLostRatio: packetsLostRatio, PacketsAcked: packetsAcked}
}
