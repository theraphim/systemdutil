package util

import (
	"net"
	"os"

	"log"
	"net/http"
	"os/signal"
	"strings"

	"github.com/coreos/go-systemd/activation"
)

type TcpOrUdp struct {
	Tcp net.Listener
	Udp net.PacketConn
	Err error
}

type Fatalf interface {
	Fatalf(format string, v ...interface{})
	Infof(format string, v ...interface{})
}

type defaultLogger struct{}

func (f defaultLogger) Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v)
}

func (f defaultLogger) Infof(format string, v ...interface{}) {
	log.Printf(format, v)
}

var Logger Fatalf = defaultLogger{}

// WrapSystemdSockets will take a list of files from Files function and create
// UDP sockets or TCP listeners for them.
func WrapSystemdSockets(files []*os.File) (result []TcpOrUdp) {
	result = make([]TcpOrUdp, len(files))
	for i, fd := range files {
		if pc, err := net.FilePacketConn(fd); err == nil {
			result[i].Udp = pc
			continue
		}
		if sc, err := net.FileListener(fd); err == nil {
			result[i].Tcp = sc
			continue
		} else {
			result[i].Err = err
		}
	}
	return
}

func Find(sockets []TcpOrUdp, start int, udp bool) int {
	for start < len(sockets) {
		if (udp && (sockets[start].Udp != nil)) || (!udp && (sockets[start].Tcp != nil)) {
			return start
		}
		start++
	}
	return -1
}

func ListenSystemd() (udps []net.PacketConn, https, grpcs []net.Listener) {
	sockets := WrapSystemdSockets(activation.Files(false))

	for _, v := range sockets {
		if v.Err != nil {
			Logger.Fatalf("systemd error: %q", v.Err)
		}
		if v.Udp != nil {
			udps = append(udps, v.Udp)
		}
		if v.Tcp != nil {
			if len(https) > 0 && len(grpcs) == 0 {
				grpcs = append(grpcs, v.Tcp)
			} else {
				https = append(https, v.Tcp)
			}
		}
	}
	return udps, https, grpcs
}

func MustListenUDPSlice(what []string) (udps []net.PacketConn) {
	for _, v := range what {
		l, err := net.ListenPacket("udp", v)
		if err != nil {
			Logger.Fatalf("ListenPacket(%q) error: %q", v, err)
		}
		udps = append(udps, l)
	}
	return udps
}

func MustListenTCPSlice(what []string) (tcps []net.Listener) {
	for _, v := range what {
		l, err := net.Listen("tcp", v)
		if err != nil {
			Logger.Fatalf("Listen(%q) error: %q", v, err)
		}
		tcps = append(tcps, l)
	}
	return tcps
}

type Server interface {
	Serve(net.Listener) error
}

func ServeAll(gs Server, https, grpcs []net.Listener) {
	for _, s := range https {
		go http.Serve(s, nil)
	}
	for _, s := range grpcs {
		go gs.Serve(s)
	}
}

func WaitSigint() {
	control := make(chan os.Signal, 1)
	signal.Notify(control, os.Interrupt)
	sig := <-control
	Logger.Infof("Exiting due to signal %s", sig)
}

func SplitListen(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
