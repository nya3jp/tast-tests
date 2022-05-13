// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing/hwdep"
)

// UniqueAPName returns AP name to be used in packet dumps.
func UniqueAPName() string {
	id := strconv.Itoa(apID)
	apID++
	return id
}

// ExpectOutput checks if string contains matching regexp.
func ExpectOutput(str, lookup string) bool {
	re := regexp.MustCompile(lookup)
	return re.MatchString(str)
}

// RunAndCheckOutput runs command and checks if the output matches expected regexp.
func RunAndCheckOutput(ctx context.Context, cmd *ssh.Cmd, lookup string) (bool, error) {
	ret, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to call command, err")
	}
	return ExpectOutput(string(ret), lookup), nil
}

// CheckTDLSSupport verifies that TDLS is supported according to the driver.
func CheckTDLSSupport(ctx context.Context, conn *ssh.Conn) error {
	ret, err := conn.CommandContext(ctx, "iw", "phy").Output()
	if err != nil {
		return errors.Wrap(err, "failed to call command, err")
	}
	if !ExpectOutput(string(ret), "tdls_oper") || !ExpectOutput(string(ret), "tdls_mgmt") {
		return errors.New("device does not declare TDLS support")
	}
	return nil
}

// GetMAC returns MAC address of the requested interface on the device accessible through SSH connection.
func GetMAC(ctx context.Context, conn *ssh.Conn, ifName string) (net.HardwareAddr, error) {
	ipr := ip.NewRemoteRunner(conn)
	hwMAC, err := ipr.MAC(ctx, ifName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MAC of WiFi interface")
	}
	return hwMAC, nil
}

// GetIPv4 returns IPv4 address of the requested interface on the device accessible through SSH connection.
func GetIPv4(ctx context.Context, conn *ssh.Conn, ifName string) (string, error) {
	ipr := ip.NewRemoteRunner(conn)
	ip, _, err := ipr.IPv4(ctx, ifName)
	if err != nil {
		return "", errors.Wrap(err, "failed to get IP of WiFi interface")
	}
	return ip.String(), nil
}

// PingFromHost pings arbirtary address from a host given by the ssh.Conn.
func PingFromHost(ctx context.Context, conn *ssh.Conn, targetIP string, opts ...ping.Option) (*ping.Result, error) {
	pr := remoteping.NewRemoteRunner(conn)
	return pr.Ping(ctx, targetIP, opts...)
}

func macEqual(a1, a2 net.HardwareAddr) bool {
	return bytes.Compare(a1, a2) == 0
}

type flagPair struct{ f1, f2 bool }

func dstAddress(dot11 *layers.Dot11) net.HardwareAddr {
	// Destination address is contextual, depending on the contents of FromDS/ToDS flags.
	addressMatrix := map[flagPair]net.HardwareAddr{
		{false, false}: dot11.Address1, {false, true}: dot11.Address3,
		{true, false}: dot11.Address1, {true, true}: dot11.Address1}
	return addressMatrix[flagPair{dot11.Flags.FromDS(), dot11.Flags.ToDS()}]
}

func srcAddress(dot11 *layers.Dot11) net.HardwareAddr {
	// Source address is contextual, depending on the contents of FromDS/ToDS flags.
	addressMatrix := map[flagPair]net.HardwareAddr{
		{false, false}: dot11.Address2, {false, true}: dot11.Address2,
		{true, false}: dot11.Address3, {true, true}: dot11.Address2}
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

// TDLSHwDeps returns dependencies for the test.
func TDLSHwDeps() hwdep.Condition {
	return hwdep.SkipOnWifiDevice(
		hwdep.Marvell88w8897SDIO, hwdep.Marvell88w8997PCIE)
}
