package congestion

import (
	"github.com/lucas-clemente/quic-go/internal/protocol"
	"time"
)

type FlowTeleCubic struct {
	Cubic

	betaRaw                         float32
	betaLastMaxRaw                  float32
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
//func (c *FlowTeleCubic) adjustBeta() {
//	c.betaRaw = c.betaValue
//	c.betaLastMaxRaw = 1 - (1-c.betaRaw)/2
//}

// TODO:	Understand why the adjust functions are needed.
//func (c *FlowTeleCubic) adjustLastMaxCongestionWindow() {
//	c.lastMaxCongestionWindow = protocol.ByteCount(int64(c.lastMaxCongestionWindow) + c.lastMaxCongestionWindowAddDelta)
//	c.lastMaxCongestionWindowAddDelta = 0
//}

func (c *FlowTeleCubic) Reset() {
	c.Cubic.Reset()
}

func (c *FlowTeleCubic) alpha() float32 {
	// TODO: 	Flowtele's original impl. uses a hardcoded beta here but then calls c.beta() in
	// 				CongestionWindowAfterPacketLoss. Which one is correct?
	// flowtele uses the hardcoded default beta value for the TCP fairness calculations
	//b := float32(0.7)
	//return 3 * float32(c.numConnections) * float32(c.numConnections) * (1 - b) / (1 + b)
	return c.Cubic.beta()
}

func (c *FlowTeleCubic) beta() float32 {
	// TODO: Compare original flowtele implementation
	//return (float32(c.numConnections) - 1 + c.betaRaw) / float32(c.numConnections)
	return c.Cubic.beta()
}

func (c *FlowTeleCubic) betaLastMax() float32 {
	// TODO: Compare original flowtele implementation
	//return (float32(c.numConnections) - 1 + c.betaLastMaxRaw) / float32(c.numConnections)
	return c.Cubic.betaLastMax()
}

func (c *FlowTeleCubic) OnApplicationLimited() {
	c.Cubic.OnApplicationLimited()
}

func (c *FlowTeleCubic) CongestionWindowAfterPacketLoss(currentCongestionWindow protocol.ByteCount) protocol.ByteCount {
	return c.Cubic.CongestionWindowAfterPacketLoss(currentCongestionWindow)
}

func (c *FlowTeleCubic) CongestionWindowAfterAck(
	ackedBytes protocol.ByteCount,
	currentCongestionWindow protocol.ByteCount,
	delayMin time.Duration,
	eventTime time.Time,
) protocol.ByteCount {
	return c.Cubic.CongestionWindowAfterAck(ackedBytes, currentCongestionWindow, delayMin, eventTime)
}

func (c *FlowTeleCubic) SetNumConnections(n int) {
	c.Cubic.SetNumConnections(n)
}
