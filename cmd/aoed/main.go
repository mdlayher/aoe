package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdlayher/aoe"
	"github.com/mdlayher/block"
)

var (
	ifaceFlag = flag.String("i", "eth0", "network interface")
	diskFlag  = flag.String("d", "/dev/sda", "storage disk")
)

func main() {
	flag.Parse()

	ifi, err := net.InterfaceByName(*ifaceFlag)
	if err != nil {
		log.Fatal(err)
	}

	block, err := block.New(*diskFlag, syscall.O_RDWR|syscall.O_CLOEXEC)
	if err != nil {
		log.Fatal(err)
	}

	s := &aoe.Server{
		Iface: ifi,

		AdvertiseInterval: 60 * time.Second,

		Major: 0x000f,
		Minor: 0x01,

		BufferCount:     0x10,
		FirmwareVersion: 0x0001,
		SectorCount:     16,

		Config: make([]byte, 0),
	}

	h := &Handler{
		block: block,
		cfg:   s.ServeConfig(),
	}
	s.Handler = h

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)

	go func() {
		log.Printf("serving ATA over Ethernet device %q on %q", *diskFlag, *ifaceFlag)
		if err := s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	sig := <-sigC
	log.Printf("caught signal: %s, exiting...", sig)
	_ = block.Close()
}

type Handler struct {
	block *block.Device

	cfg aoe.Handler
}

func (h *Handler) ServeAoE(w aoe.ResponseSender, r *aoe.Request) {
	/*
		if r.Header.Command != aoe.CommandIssueATACommand {
			log.Printf("[%s -> %s] header: %+v", r.Source, r.Target, r.Header, r.Header.Arg)
		}
	*/

	switch r.Header.Command {
	case aoe.CommandQueryConfigInformation:
		h.cfg.ServeAoE(w, r)
		return
	case aoe.CommandIssueATACommand:
		if _, err := aoe.ServeATA(w, r.Header, h.block); err != nil {
			log.Println("ATA error:", err)
		}
		return
	}
}

/*
	-static ushort ident[256] = {
		-       [47] 0x8000,
		-       [49] 0x0200,
		-       [50] 0x4000,
		-       [83] 0x5400,
		-       [84] 0x4000,
		-       [86] 0x1400,
		-       [87] 0x4000,
		-       [93] 0x400b,
		-};
*/
