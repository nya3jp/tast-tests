// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"context"
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
)

// CheckTDLSSupport verifies that TDLS is supported according to the driver.
func CheckTDLSSupport(ctx context.Context, conn *ssh.Conn) error {
	phys, _, err := remoteiw.NewRemoteRunner(conn).ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get capabilities")
	}
	// Basing on two assumptions:
	// 1. All phys of the same modem will have the same capabilities.
	// 2. We support only one WiFi modem per DUT.
	checkCommand := func(phys []*iw.Phy, command string) bool {
		for _, c := range phys[0].Commands {
			if c == command {
				return true
			}
		}
		return false
	}
	if checkCommand(phys, "tdls_oper") && checkCommand(phys, "tdls_mgmt") {
		return nil
	}
	return errors.New("Device does not declare full TDLS support")
}

// Scan facilitates running iw scan.
func Scan(ctx context.Context, conn *ssh.Conn, ifName, ssid string) error {
	_, err := remoteiw.NewRemoteRunner(conn).TimedScan(ctx, ifName, nil, []string{ssid})
	return err
}

func macEqual(a1, a2 net.HardwareAddr) bool {
	return bytes.Compare(a1, a2) == 0
}

type flagPair struct{ f1, f2 bool }

func dstAddress(dot11 *layers.Dot11) net.HardwareAddr {
	// Destination address is contextual, depending on the contents of FromDS/ToDS flags.
	addressMatrix := map[flagPair]net.HardwareAddr{
		{false, false}: dot11.Address1, {false, true}: dot11.Address3,
		{true, false}: dot11.Address1, {true, true}: dot11.Address3}
	return addressMatrix[flagPair{dot11.Flags.FromDS(), dot11.Flags.ToDS()}]
}

func srcAddress(dot11 *layers.Dot11) net.HardwareAddr {
	// Source address is contextual, depending on the contents of FromDS/ToDS flags.
	addressMatrix := map[flagPair]net.HardwareAddr{
		{false, false}: dot11.Address2, {false, true}: dot11.Address2,
		{true, false}: dot11.Address3, {true, true}: dot11.Address4}
	return addressMatrix[flagPair{dot11.Flags.FromDS(), dot11.Flags.ToDS()}]
}

// filterTrnsRecv filters packets that have given src/dst addresses, but different Transmitter/Receiver addresses.
func filterTrnsRecv(dot11 *layers.Dot11, src, dst net.HardwareAddr) bool {
	if !macEqual(dstAddress(dot11), dst) || !macEqual(srcAddress(dot11), src) {
		// These packets are not interesting.
		return false
	}
	if !macEqual(dot11.Address1, dst) || !macEqual(dot11.Address2, src) {
		// If Transmitter/Receiver is different than src/dst, we want to see this packet.
		return true
	}
	// Correct packet, filter out.
	return false
}

// FindNonTDLSPackets finds ICMP packets that were not sent through TDLS.
func FindNonTDLSPackets(pcapPath string, addrs []net.HardwareAddr) ([]gopacket.Packet, error) {
	if len(addrs) != 2 {
		return nil, errors.New("Needs exactly two addresses")
	}
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(layers.LayerTypeICMPv4,
			func(layer gopacket.Layer) bool { return true }),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				if dot11.Type.MainType() != layers.Dot11TypeData {
					return false
				}
				return filterTrnsRecv(dot11, addrs[0], addrs[1]) ||
					filterTrnsRecv(dot11, addrs[1], addrs[0])
			},
		),
	}
	return pcap.ReadPackets(pcapPath, filters...)
}

// DumpPkts dissects slice of packets into a string.
func DumpPkts(pkts []gopacket.Packet) string {
	var ret string
	for i, pkt := range pkts {
		ret = ret + fmt.Sprintf("%d: %s\n", i, pkt.Dump())
	}
	return ret
}
