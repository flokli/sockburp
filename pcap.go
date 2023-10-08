package main

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
)

type PcapWriter struct {
	listenIf int
	firstIf  int
	secondIf int

	pcapWriter *pcapgo.NgWriter
}

func (p *PcapWriter) WritePacket(req, resp1, resp2 []byte) error {
	if err := p.pcapWriter.WritePacket(gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		CaptureLength:  len(req),
		Length:         len(req),
		InterfaceIndex: p.listenIf,
	}, req); err != nil {
		return fmt.Errorf("unable to write req packet: %w", err)
	}

	if err := p.pcapWriter.WritePacket(gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		CaptureLength:  len(resp1),
		Length:         len(resp1),
		InterfaceIndex: p.firstIf,
	}, resp1); err != nil {
		return fmt.Errorf("unable to write first reply packet: %w", err)
	}

	if err := p.pcapWriter.WritePacket(gopacket.CaptureInfo{
		Timestamp:      time.Now(),
		CaptureLength:  len(resp2),
		Length:         len(resp2),
		InterfaceIndex: p.secondIf,
	}, resp2); err != nil {
		return fmt.Errorf("unable to write second reply packet: %w", err)
	}

	return nil
}

func (p *PcapWriter) Flush() error {
	return p.pcapWriter.Flush()
}

func (p *PcapWriter) Close() error {
	if err := p.pcapWriter.Flush(); err != nil {
		return fmt.Errorf("unable to flush pcap file: %w", err)
	}

	return nil
}

func NewPcapWriter(w io.Writer) (*PcapWriter, error) {
	pcapWriter, err := pcapgo.NewNgWriter(w, 147) // DLT_USER0
	if err != nil {
		return nil, fmt.Errorf("unable to open pcap ng writer: %w", err)
	}

	listenIf, err := pcapWriter.AddInterface(pcapgo.NgInterface{
		Name:     cli.ListenAddr,
		Comment:  "listen address",
		OS:       runtime.GOOS,
		LinkType: 147,
	})

	firstIf, err := pcapWriter.AddInterface(pcapgo.NgInterface{
		Name:     cli.FirstRemoteAddr,
		Comment:  "first remote address",
		OS:       runtime.GOOS,
		LinkType: 147,
	})

	secondIf, err := pcapWriter.AddInterface(pcapgo.NgInterface{
		Name:     cli.SecondRemoteAddr,
		Comment:  "second remote address",
		OS:       runtime.GOOS,
		LinkType: 147,
	})

	return &PcapWriter{
		listenIf:   listenIf,
		firstIf:    firstIf,
		secondIf:   secondIf,
		pcapWriter: pcapWriter,
	}, nil
}
