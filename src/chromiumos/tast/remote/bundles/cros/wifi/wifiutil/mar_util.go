// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"context"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

// isBroadcastMAC checks if MAC Address is a broadcast one.
func isBroadcastMAC(mac net.HardwareAddr) bool {
	return bytes.Compare(mac, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) == 0
}

// findWrongPackets is a simple filter returning all packets from capture
// indicated by pcapPath that fail to pass the wrongMAC check, that is when
// wrongMAC function returns true.
func findWrongPackets(pcapPath string, wrongMAC func(mac net.HardwareAddr, toDS bool) bool) ([]gopacket.Packet, error) {
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				if dot11.Flags.ToDS() {
					return wrongMAC(dot11.Address2, true)
				}
				if dot11.Flags.FromDS() {
					return (!isBroadcastMAC(dot11.Address1) && wrongMAC(dot11.Address1, false)) ||
						(!isBroadcastMAC(dot11.Address3) && wrongMAC(dot11.Address3, false))
				}
				return false
			},
		),
	}
	return pcap.ReadPackets(pcapPath, filters...)
}

// VerifyMACIsKept checks whether currently used MAC address (macAddr) is the
// same as the original one (origMAC) and that this address is used in all
// communication (by parsing captured packets in pcapPath).
func VerifyMACIsKept(ctx context.Context, macAddr net.HardwareAddr, pcapPath string, origMAC net.HardwareAddr) error {
	var wrongMACUsed net.HardwareAddr
	wrongMAC := func(mac net.HardwareAddr, toDS bool) bool {
		// When checking that all packets use original MAC we have to
		// confine search to only 'toDS' direction because of high
		// probability of getting unrelated packet from the environment.
		if !bytes.Equal(mac, origMAC) && toDS {
			if wrongMACUsed == nil {
				wrongMACUsed = mac
			}
			return true
		}
		return false
	}
	if wrongMAC(macAddr, true) {
		return errors.Errorf("hardware address changed: got %s, want %s", macAddr, origMAC)
	}
	packets, err := findWrongPackets(pcapPath, wrongMAC)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) > 0 {
		testing.ContextLogf(ctx, "Found %d packets with incorrect MAC", len(packets))
		return errors.New("found packet using wrong MAC: " + wrongMACUsed.String())
	}
	return nil
}

// VerifyMACIsChanged checks whether currently used MAC address (macAddr) is
// different from any previously used addresses (indicated in prevMACs).
// It also verifies that none of the previously used MAC addresses is used in
// communication (by parsing captured packets in pcapPath).
func VerifyMACIsChanged(ctx context.Context, macAddr net.HardwareAddr, pcapPath string, prevMACs []net.HardwareAddr) error {
	var prevMACUsed net.HardwareAddr
	wrongMAC := func(mac net.HardwareAddr, _ bool) bool {
		for _, prevMAC := range prevMACs {
			if bytes.Equal(mac, prevMAC) {
				if prevMACUsed == nil {
					prevMACUsed = prevMAC
				}
				return true
			}
		}
		return false
	}
	if wrongMAC(macAddr, true) {
		return errors.New("used previous MAC address: " + prevMACUsed.String())
	}
	packets, err := findWrongPackets(pcapPath, wrongMAC)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) > 0 {
		testing.ContextLogf(ctx, "Found %d packets with incorrect MAC", len(packets))
		return errors.New("found packet with previously used MAC: " + prevMACUsed.String())
	}
	return nil
}
