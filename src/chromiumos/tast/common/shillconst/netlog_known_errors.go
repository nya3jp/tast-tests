// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shillconst

// AllowedEntry defines the error log entry that is allowed/expected.
type AllowedEntry struct {
	Program      string
	FileName     string
	MessageRegex string
	Counter      int
}

// InitializeAllowedEntries returns the allowed log entries with Counter = 0.
func InitializeAllowedEntries() []AllowedEntry {
	return []AllowedEntry{
		{"dhcpcd", "", ".*eth\\d: checksum failure from.*", 0},
		{"dhcpcd", "", ".*eth\\d: DHCP lease expired.*", 0},
		{"dhcpcd", "", ".*eth\\d: dhcp_envoption 119: Operation not supported.*", 0},
		{"dhcpcd", "", ".*eth\\d: truncated packet.*", 0},
		{"dnsproxyd", "client.cc", ".*Unable to get properties for device.*", 0},
		{"dnsproxyd", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"dnsproxyd", "object_proxy.cc", ".*Failed to call method: org.chromium.flimflam.Device.GetProperties.*", 0},
		{"ModemManager", "", ".*SIM is missing and SIM hot swap is configured, but ports are not opened.*", 0},
		{"patchpaneld", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"patchpaneld", "manager.cc", ".*Invalid namespace name.*", 0},
		{"patchpaneld", "net_util.cc", ".*Invalid prefix length.*", 0},
		{"patchpaneld", "network_monitor_service.cc", ".*Get device props failed.*", 0},
		{"patchpaneld", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"patchpaneld", "scoped_ns.cc", ".*Could not open namespace.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Can't retrieve properties for device.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Empty interface name for shill Device \\/device\\/eth\\d.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Failed to obtain service.*GetProperties.*signature.*doesn't exist.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unable to get manager properties.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unknown interface name eth\\d.*", 0},
		{"shill", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"shill", "device_info.cc", ".*Add Link message for.*does not have .*", 0},
		{"shill", "dns_client.cc", ".*No valid DNS server addresses.*", 0},
		{"shill", "eap_listener.cc", ".*Could not bind socket to interface: No such device.*", 0},
		{"shill", "eap_listener.cc", ".*Could not open EAP listener socket.*", 0},
		{"shill", "ethernet.cc", ".*cannot disable \\[18\\] tx-tcp-ecn-segmentation.*", 0},
		{"shill", "ethernet.cc", ".*OnSetInterfaceMacResponse received response with error Cannot assign requested address.*", 0},
		{"shill", "http_request.cc", ".*Failed to start DNS client.*", 0},
		{"shill", "netlink_manager.cc", ".*Unexpected auxilliary message type: 0.*", 0},
		{"shill", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"shill", "portal_detector.cc", ".*HTTP probe failed to start.*", 0},
		{"shill", "rtnl_handler.cc", ".*Cannot assign requested address.*", 0},
		{"shill", "rtnl_handler.cc", ".*sequence.*received error 3 \\(No such process\\).*", 0},
		{"shill", "supplicant_interface_proxy.cc", ".*Failed to scan: fi.w1.wpa_supplicant1.Interface.ScanError Scan request rejected.*", 0},
		{"shill", "supplicant_process_proxy.cc", ".*Failed to create interface: fi.w1.wpa_supplicant1.UnknownError.*", 0},
		{"shill", "supplicant_process_proxy.cc", ".*Failed to get interface wlan0: fi.w1.wpa_supplicant1.InterfaceUnknown.*", 0},
		{"shill", "unknown", ".*", 0},
		{"shill", "userdb_utils.cc", ".*Unable to find user.*", 0},
		{"shill", "utils.cc", ".*AddDBusError.*", 0},
		{"shill", "wifi.cc", ".*ConnectToSupplicant: Failed to create interface with supplicant.*", 0},
		{"shill", "wifi.cc", ".*does not support MAC address randomization.*", 0},
		{"shill", "wifi.cc", ".*Unsupported NL80211_ATTR_REG_ALPHA2 attribute: 99.*", 0},
		{"wpa_supplicant", "", ".*Could not set interface wlan0 flags \\(UP\\): Input\\/output error.*", 0},
		{"wpa_supplicant", "", ".*nl80211: Could not set interface 'wlan0' UP.*", 0},
		{"wpa_supplicant", "", ".*Permission denied.*", 0},
		{"wpa_supplicant", "", ".*wlan0: Failed to initialize driver interface.*", 0},
	}
}
