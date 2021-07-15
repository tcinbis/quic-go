package flowtele

import (
	"time"
)

func CreateFlowteleSignalInterface(newSrttMeasurement func(t time.Time, srtt time.Duration), packetsLost func(t time.Time, newSlowStartThreshold uint64), packetsLostRatio func(t time.Time, lostRatio float64), packetsAcked func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64)) *FlowTeleSignal {
	return &FlowTeleSignal{NewSrttMeasurement: newSrttMeasurement, PacketsLost: packetsLost, PacketLostRatio: packetsLostRatio, PacketsAcked: packetsAcked}
}
