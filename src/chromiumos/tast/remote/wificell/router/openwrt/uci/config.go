// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uci

// Constants for relevant config file locations as specified in the OpenWrt
// documentation here: https://openwrt.org/docs/guide-user/base-system/uci#configuration_files.
const (
	// ConfigDhcp is the path to the config file for dnsmasq and odhcpd
	// settings: DNS, DHCP, DHCPv6.
	ConfigDhcp string = "/etc/config/dhcp"

	// ConfigNetwork is the path to the config file for switch, interface and
	// route configuration: Basics, IPv4, IPv6, Routes, Rules, WAN, Aliases,
	// Switches, VLAN, IPv4/IPv6 transitioning, Tunneling.
	ConfigNetwork = "/etc/config/network"

	// ConfigWireless is the path to the config file for wireless settings and
	// Wi-Fi network definition.
	ConfigWireless = "/etc/config/wireless"
)
