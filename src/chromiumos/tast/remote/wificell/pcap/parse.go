// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"bytes"
	"io"
	"net"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"chromiumos/tast/errors"
)

// This file contains some utilities for reading and filtering WiFi packets
// from pcap files.

// Filter is the function type for filtering packets.
// The packet should be dropped if it returns false.
type Filter func(gopacket.Packet) bool

// TypeFilter returns a Filter which ensures the packet contains a Layer with
// type t and passes the check function on the layer if check() is given.
func TypeFilter(t gopacket.LayerType, check func(gopacket.Layer) bool) Filter {
	return func(p gopacket.Packet) bool {
		layer := p.Layer(t)
		if layer == nil {
			return false
		}
		return check == nil || check(layer)
	}
}

// RejectLowSignal returns a Filter which ensures the signal strength is good
// enough (greater than -85 dBm).
func RejectLowSignal() Filter {
	return TypeFilter(layers.LayerTypeRadioTap,
		func(layer gopacket.Layer) bool {
			radioTap := layer.(*layers.RadioTap)
			return radioTap.DBMAntennaSignal > -85
		})
}

// Dot11FCSValid returns a Filter which ensures the frame check sequence of
// the 802.11 frame is valid.
func Dot11FCSValid() Filter {
	return TypeFilter(layers.LayerTypeDot11,
		func(layer gopacket.Layer) bool {
			dot11 := layer.(*layers.Dot11)
			return dot11.ChecksumValid()
		})
}

// TransmitterAddress returns a Filter which ensures the Transmitter Address
// matches the given MAC address.
func TransmitterAddress(mac net.HardwareAddr) Filter {
	return TypeFilter(layers.LayerTypeDot11,
		func(layer gopacket.Layer) bool {
			dot11 := layer.(*layers.Dot11)
			return bytes.Equal(dot11.Address2, mac)
		})
}

// readPacketsFromReader reads packets from io.Reader of a pcap file and returns
// the ones which pass all the filters.
func readPacketsFromReader(r io.Reader, filters ...Filter) ([]gopacket.Packet, error) {
	reader, err := pcapgo.NewReader(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pcapgo reader")
	}
	source := gopacket.NewPacketSource(reader, reader.LinkType())

	var ret []gopacket.Packet

	passFilters := func(p gopacket.Packet) bool {
		for _, f := range filters {
			if !f(p) {
				return false
			}
		}
		return true
	}
	for p := range source.Packets() {
		if passFilters(p) {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

// ReadPackets reads packets from a pcap file and returns the ones which pass
// all the filters.
func ReadPackets(pcapFile string, filters ...Filter) ([]gopacket.Packet, error) {
	f, err := os.Open(pcapFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", pcapFile)
	}
	defer f.Close()
	return readPacketsFromReader(f, filters...)
}

// ParseProbeReqSSID parses the frame body of a probe request packet and
// returns the SSID in the request.
func ParseProbeReqSSID(req *layers.Dot11MgmtProbeReq) (string, error) {
	// LayerContents of probe request is the frame body.
	content := req.LayerContents()
	// Parse the content as information elements.
	e := gopacket.NewPacket(content, layers.LayerTypeDot11InformationElement, gopacket.NoCopy)
	if err := e.ErrorLayer(); err != nil {
		return "", errors.Wrap(err.Error(), "failed to parse information elements")
	}
	for _, l := range e.Layers() {
		element, ok := l.(*layers.Dot11InformationElement)
		if !ok {
			return "", errors.Errorf("unexpected layer %v", l)
		}
		if element.ID == layers.Dot11InformationElementIDSSID {
			return string(element.Info), nil
		}
	}
	return "", errors.New("no SSID element found")
}
