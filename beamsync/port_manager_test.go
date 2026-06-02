package beamsync

import (
	"net"
	"testing"
)

func TestFindAvailablePortSkipsBusyPorts(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen on random port: %v", err)
	}
	defer busy.Close()

	startPort := busy.Addr().(*net.TCPAddr).Port
	port, listener, err := FindAvailablePort(startPort, 1, 3)
	if err != nil {
		t.Fatalf("FindAvailablePort returned error: %v", err)
	}
	defer listener.Close()

	if port == startPort {
		t.Fatalf("FindAvailablePort returned busy port %d", port)
	}
}

func TestFindAvailablePortReportsExhaustion(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen on random port: %v", err)
	}
	defer busy.Close()

	startPort := busy.Addr().(*net.TCPAddr).Port
	port, listener, err := FindAvailablePort(startPort, 1, 1)

	if err == nil {
		if listener != nil {
			listener.Close()
		}
		t.Fatal("expected exhaustion error")
	}
	if port != 0 || listener != nil {
		t.Fatalf("port=%d listener=%v, want zero/nil on failure", port, listener)
	}
}
