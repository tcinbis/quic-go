package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/flowtele"
	"github.com/lucas-clemente/quic-go/interop/utils"
)

var (
	localIPFlag   = flag.String("local-ip", "", "IP address to listen on.")
	localPortFlag = flag.Int("local-port", 5500, "Port number to listen on.")
)

const (
	Bit  = 1
	KBit = 1000 * Bit
	MBit = 1000 * KBit
)

func main() {
	startSession()
}

func setupFlowTeleSignaling() *flowtele.FlowTeleSignal {
	newSrttMeasurement := func(t time.Time, srtt time.Duration) {
		// fmt.Printf("SRTT: %s\n", t.Format("2006-01-02 15:04:05"), srtt.String())
	}
	packetsLost := func(t time.Time, newSlowStartThreshold uint64) {
		// fmt.Printf("Packet LOST at %d.\n", t.Format("2006-01-02 15:04:05"))
	}
	packetsLostRatio := func(t time.Time, lostRatio float64) {}
	packetsAcked := func(t time.Time, congestionWindow uint64, packetsInFlight uint64, ackedBytes uint64) {
		// fmt.Printf("ACKED \t cwnd: %d \t inFlight: %d \t ackedBytes: %d\n", congestionWindow, packetsInFlight, ackedBytes)
	}

	return flowtele.CreateFlowteleSignalInterface(newSrttMeasurement, packetsLost, packetsLostRatio, packetsAcked)
}

func startSession() {
	quicConf := quic.Config{
		KeepAlive:      true,
		FlowTeleSignal: setupFlowTeleSignaling(),
	}

	lAddr := &net.UDPAddr{IP: net.ParseIP(*localIPFlag), Port: *localPortFlag}
	fmt.Printf("Listening on: %s\n", lAddr)
	conn, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		fmt.Printf("Error starting UDP listener: %s\n", err)
		return
	}

	listener, err := quic.Listen(conn, generateTLSConfig(), &quicConf)
	if err != nil {
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			log.Printf("Connection timed out")
			return
		} else {
			log.Printf("%v\n", err)
			return
		}
	}

	session, err := listener.Accept(context.Background())
	if err != nil {
		log.Printf("Accept error: %v\n", err)
	}
	fmt.Printf("Session established.\n")
	fsession := utils.CheckFlowTeleSession(session)
	fmt.Printf("Waiting for streams...\n")
	stream, err := fsession.AcceptStream(context.Background())
	if err != nil {
		checkQuicError("stream", err)
	}
	fmt.Printf("Stream opened.\n")

	// continuously send 10MB messages to quic listener
	message := make([]byte, 10000000)
	for i := range message {
		message[i] = 42
	}
	rate := uint64((10 * MBit * 0.8))
	fmt.Printf("Setting rate to %d\n", rate)
	fsession.SetFixedRate(rate)
	for {
		_, err = stream.Write(message)
		if err != nil {
			fmt.Printf("Error writing message to [%s]: %s\n", fsession.RemoteAddr(), err)
			return
		}
	}
}

// checks for common quic errors, returns true if okay (no error)
func checkQuicError(errItem string, err error) bool {
	if err != nil {
		if err == io.EOF {
			log.Printf("%s end of file.\n", errItem)
		} else if err == io.ErrClosedPipe {
			log.Printf("%s closed.\n", errItem)
		} else if netError, ok := err.(net.Error); netError.Timeout() && ok {
			log.Printf("%s timed out.\n", errItem)
		} else {
			log.Printf("UNKNOWN ERROR: for %s; %v", errItem, err)
			panic(err)
		}
		return false
	}
	return true
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-example"},
	}
}
