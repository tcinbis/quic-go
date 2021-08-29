package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const messageSize = 10000000

var (
	remoteIPFlag   = flag.String("ip", "127.0.0.1", "IP address to connect to.")
	remotePortFlag = flag.Int("port", 5500, "Port number to connect to.")
	localIPFlag    = flag.String("local-ip", "", "IP address to listen on.")
	localPortFlag  = flag.Int("local-port", 0, "Port number to listen on.")
)

func main() {
	flag.Parse()
	rAddr := &net.UDPAddr{IP: net.ParseIP(*remoteIPFlag), Port: *remotePortFlag}
	lAddr := &net.UDPAddr{IP: net.ParseIP(*localIPFlag), Port: *localPortFlag}
	startSession(rAddr, lAddr)
}

func startSession(rAddr *net.UDPAddr, lAddr *net.UDPAddr) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}
	quicConf := quic.Config{
		KeepAlive: true,
	}
	fmt.Printf("Connecting to remote address: %s\n", rAddr)
	conn, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		fmt.Printf("Error listening UDP: %s\n", err)
		return
	}
	session, err := quic.Dial(conn, rAddr, "host:0", tlsConf, &quicConf)
	if err != nil {
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			log.Printf("Connection timed out")
			return
		} else {
			log.Printf("Session dial error: %v\n", err)
			return
		}
	}
	fmt.Println("Session established.")
	time.Sleep(2 * time.Second)
	fmt.Println("Opening stream...")

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		checkQuicError("Stream", err)
	}
	fmt.Printf("Stream opened.\n")
	_, err = stream.Write([]byte("Hello"))
	if err != nil {
		fmt.Printf("Error writing: %v\n", err)
	}
	checkQuicError("Listening", listenOnStream(session, stream))
}

func getPort(addr net.Addr) (int, error) {
	switch addr := addr.(type) {
	case *net.UDPAddr:
		return addr.Port, nil
	default:
		return 0, fmt.Errorf("unknown address type")
	}
}

func listenOnStream(session quic.Session, stream quic.Stream) error {
	defer session.CloseWithError(0x100, "")
	message := make([]byte, messageSize)
	tInit := time.Now()
	nTot := 0
	rPort, err := getPort(session.RemoteAddr())
	if err != nil {
		fmt.Printf("Error resolving remote UDP address: %s\n", err)
		return err
	}
	lPort, err := getPort(session.LocalAddr())
	if err != nil {
		fmt.Printf("Error resolving local UDP address: %s\n", err)
		return err
	}
	fmt.Printf("%d_%d: Listening on Stream %d\n", lPort, rPort, stream.StreamID())
	for {
		tStart := time.Now()
		n, err := io.ReadFull(stream, message)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				// sender stopped sending
				return nil
			} else {
				return fmt.Errorf("error reading message: %s", err)
			}
		}
		tEnd := time.Now()
		nTot += n
		tCur := tEnd.Sub(tStart).Seconds()
		tTot := tEnd.Sub(tInit).Seconds()
		// Mbit/s
		curRate := float64(n) / tCur / 1000000.0 * 8.0
		totRate := float64(nTot) / tTot / 1000000.0 * 8.0
		fmt.Printf("%d_%d cur: %.1fMbit/s (%.1fMB in %.2fs), tot: %.1fMbit/s (%.1fMB in %.2fs)\n", lPort, rPort, curRate, float64(n)/1000000, tCur, totRate, float64(nTot)/1000000, tTot)
	}
}

// checks for common quic errors, returns true if okay (no error)
func checkQuicError(errItem string, err error) {
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
	}
}
