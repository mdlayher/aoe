package aoe

import (
	"io"
	"net"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/raw"
)

type Handler interface {
	ServeAoE(ResponseSender, *Request)
}

type HandlerFunc func(ResponseSender, *Request)

func (f HandlerFunc) ServeAoE(w ResponseSender, r *Request) {
	f(w, r)
}

type Request struct {
	Source net.HardwareAddr
	Target net.HardwareAddr
	Header *Header
}

type Server struct {
	Iface *net.Interface

	AdvertiseInterval time.Duration

	Major uint16
	Minor uint8

	BufferCount     uint16
	FirmwareVersion uint16
	SectorCount     uint8

	Config []byte

	Handler Handler

	p net.PacketConn
}

func ListenAndServe(iface string, handler Handler) error {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return err
	}

	return (&Server{
		Iface:   ifi,
		Handler: handler,
	}).ListenAndServe()
}

func (s *Server) ListenAndServe() error {
	p, err := raw.ListenPacket(s.Iface, syscall.ETH_P_AOE)
	if err != nil {
		return err
	}

	return s.Serve(p)
}

func (s *Server) advertiseLoop(ctx context.Context) {
	tick := time.NewTicker(s.AdvertiseInterval)
	defer tick.Stop()

	for {
		if _, err := s.advertise(ethernet.Broadcast); err != nil {
			// TODO(mdlayher): log or handle error
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) Serve(p net.PacketConn) error {
	s.p = p
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.advertiseLoop(ctx)

	// Loop and read requests until exit
	buf := make([]byte, 2048)
	for {
		n, addr, err := s.p.ReadFrom(buf)
		if err != nil {
			// Treat EOF as an exit signal
			if err == io.EOF {
				return nil
			}

			return err
		}

		s.newConn(addr.(*raw.Addr), n, buf).serve()
	}
}

func (s *Server) ServeConfig() Handler {
	return HandlerFunc(func(w ResponseSender, r *Request) {
		_, _ = w.Send(&Header{
			Arg: &ConfigArg{
				BufferCount:     s.BufferCount,
				FirmwareVersion: s.FirmwareVersion,
				SectorCount:     s.SectorCount,
				Version:         Version,
				Command:         ConfigCommandRead,
				StringLength:    uint16(len(s.Config)),
				String:          s.Config,
			},
		})
	})
}

func (s *Server) advertise(target net.HardwareAddr) (int, error) {
	h := &Header{
		Version:      Version,
		FlagResponse: true,
		Major:        s.Major,
		Minor:        s.Minor,
		Command:      CommandQueryConfigInformation,
		Tag:          [4]byte{},
		Arg: &ConfigArg{
			BufferCount:     s.BufferCount,
			FirmwareVersion: s.FirmwareVersion,
			SectorCount:     s.SectorCount,
			Version:         Version,
			Command:         ConfigCommandRead,
			StringLength:    uint16(len(s.Config)),
			String:          s.Config,
		},
	}

	return s.send(h, s.Iface.HardwareAddr, target)
}

func (s *Server) send(h *Header, source net.HardwareAddr, target net.HardwareAddr) (int, error) {
	hb, err := h.MarshalBinary()
	if err != nil {
		return 0, err
	}

	f := &ethernet.Frame{
		Destination: target,
		Source:      source,
		EtherType:   EtherType,
		Payload:     hb,
	}

	fb, err := f.MarshalBinary()
	if err != nil {
		return 0, err
	}

	return s.p.WriteTo(fb, &raw.Addr{
		HardwareAddr: target,
	})
}

// A conn is an in-flight ARP request which contains information about a
// request to the server.
type conn struct {
	s *Server

	remoteAddr *raw.Addr
	buf        []byte
}

// newConn creates a new conn using information received in a single ARP
// request.  newConn makes a copy of the input buffer for use in handling
// a single connection.
func (s *Server) newConn(addr *raw.Addr, n int, buf []byte) *conn {
	c := &conn{
		s: s,

		remoteAddr: addr,
		buf:        make([]byte, n),
	}
	copy(c.buf, buf[:n])

	return c
}

// serve handles serving an individual ARP request, and is invoked in a
// goroutine.
func (c *conn) serve() {
	f := new(ethernet.Frame)
	if err := f.UnmarshalBinary(c.buf); err != nil {
		return
	}
	if f.EtherType != EtherType {
		return
	}

	h := new(Header)
	if err := h.UnmarshalBinary(f.Payload); err != nil {
		return
	}
	if h.FlagResponse {
		return
	}
	if h.Major != BroadcastMajor && h.Major != c.s.Major {
		return
	}
	if h.Minor != BroadcastMinor && h.Minor != c.s.Minor {
		return
	}

	// Set up response to send data back to client
	w := &response{
		s: c.s,

		localAddr:  c.s.Iface.HardwareAddr,
		remoteAddr: c.remoteAddr,

		major: c.s.Major,
		minor: c.s.Minor,

		r: h,
	}

	// If set, invoke ARP handler using request and response
	// Default to DefaultServeMux if handler is not available
	handler := c.s.Handler
	if handler == nil {
		return
		//handler = DefaultServeMux
	}

	handler.ServeAoE(w, &Request{
		Source: c.remoteAddr.HardwareAddr,
		Target: c.s.Iface.HardwareAddr,
		Header: h,
	})
}

// response represents an ARP response, and implements ResponseSender so that
// outbound Packets can be appropriately created and sent to a client.
type response struct {
	s *Server

	localAddr  net.HardwareAddr
	remoteAddr *raw.Addr

	major uint16
	minor uint8

	r *Header
}

// Send marshals an input Packet to binary form, wraps it in an ethernet frame,
// and sends it to the hardware address specified by r.remoteAddr.
func (w *response) Send(h *Header) (int, error) {
	// Outgoing traffic is always a Response
	h.Version = Version
	h.FlagResponse = true

	h.Major = w.major
	h.Minor = w.minor

	h.Command = w.r.Command
	h.Tag = w.r.Tag

	//log.Printf("send: %+v %+v", h, h.Arg)

	return w.s.send(h, w.localAddr, w.remoteAddr.HardwareAddr)
}
