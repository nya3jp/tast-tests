// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pcap

import (
	"io"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"chromiumos/tast/errors"
)

// This file contains some utilities for reading and filtering WiFi packets
// from pcap files.

// Filter is the function prototype for filtering packets.
// If false is returned, it means that the packet should be dropped.
type Filter func(gopacket.Packet) bool

// TypeFilter returns a Filter which ensure the packet contains a Layer
// with type t and passes the check function on the layer if non-nil check
// is given.
func TypeFilter(t gopacket.LayerType, check func(gopacket.Layer) bool) Filter {
	return func(p gopacket.Packet) bool {
		layer := p.Layer(t)
		if layer == nil {
			return false
		}
		if check == nil {
			return true
		}
		return check(layer)
	}
}

// RejectLowSignal returns a Filter which ensure the signal strength is good
// enough (greater than -85 dBm).
func RejectLowSignal() Filter {
	return TypeFilter(layers.LayerTypeRadioTap,
		func(layer gopacket.Layer) bool {
			radioTap := layer.(*layers.RadioTap)
			return radioTap.DBMAntennaSignal > -85
		})
}

// Dot11FCSValid returns a Filter which ensure the frame check sequence of
// the 802.11 frame is valid.
func Dot11FCSValid() Filter {
	return TypeFilter(layers.LayerTypeDot11,
		func(layer gopacket.Layer) bool {
			dot11 := layer.(*layers.Dot11)
			return dot11.ChecksumValid()
		})
}

// ReadPacketsFromReader reads packets from io.Reader to a pcap file and return the
// ones which pass all the filters.
func ReadPacketsFromReader(r io.Reader, filters ...Filter) ([]gopacket.Packet, error) {
	reader, err := pcapgo.NewReader(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pcapgo reader")
	}
	source := gopacket.NewPacketSource(reader, reader.LinkType())

	var ret []gopacket.Packet
packetLoop:
	for p := range source.Packets() {
		for _, f := range filters {
			if !f(p) {
				continue packetLoop
			}
		}
		ret = append(ret, p)
	}
	return ret, nil
}

// ReadPackets reads packets from a pcap file and return the ones which pass
// all the filters.
func ReadPackets(pcapFile string, filters ...Filter) ([]gopacket.Packet, error) {
	f, err := os.Open(pcapFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", pcapFile)
	}
	defer f.Close()
	return ReadPacketsFromReader(f, filters...)
}
