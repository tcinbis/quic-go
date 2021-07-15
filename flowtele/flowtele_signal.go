package flowtele

import "time"

type FlowTeleSignal struct {
	NewSrttMeasurement func(t time.Time, srtt time.Duration)
	PacketsLost        func(t time.Time, newSlowStartThreshold uint64)
	PacketLostRatio    func(t time.Time, lostRatio float64)
	PacketsAcked       func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64)
}
