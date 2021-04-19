package flowtele

import "time"

// TODO: Check if this is the proper location for the struct
type FlowTeleSignal struct {
	NewSrttMeasurement func(t time.Time, srtt time.Duration)
	PacketsLost        func(t time.Time, newSlowStartThreshold uint64)
	PacketsAcked       func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64)
}
