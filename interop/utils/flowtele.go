package utils

import "github.com/lucas-clemente/quic-go"

func CheckFlowTeleSession(s quic.Session) quic.FlowTeleSession {
	fs, ok := s.(quic.FlowTeleSession)
	if !ok {
		panic("Returned session is not flowtele sessions")
	}
	return fs
}
