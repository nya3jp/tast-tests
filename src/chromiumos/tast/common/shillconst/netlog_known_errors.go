// Copyright 2021 The ChromiumOS Authors
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
		{"dhcpcd", "", ".*eth.*: truncated packet.*", 0},
		{"dnsproxyd", "client.cc", ".*Unable to get properties for device.*", 0},
		{"dnsproxyd", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"dnsproxyd", "object_proxy.cc", ".*Failed to call method: org.chromium.flimflam.Device.GetProperties.*", 0},
		{"ModemManager", "", ".*SIM is missing and SIM hot swap is configured, but ports are not opened.*", 0},
		{"patchpaneld", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"patchpaneld", "manager.cc", ".*Invalid namespace name.*", 0},
		{"patchpaneld", "ndproxy.cc", ".*failed to get interface name on interface.*No such device.*", 0}, // b/237298950
		{"patchpaneld", "net_util.cc", ".*Invalid prefix length.*", 0},
		{"patchpaneld", "network_monitor_service.cc", ".*Get device props failed.*", 0},
		{"patchpaneld", "network_monitor_service.cc", ".*Could not obtain interface index for.*", 0}, // b/255732860
		{"patchpaneld", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"patchpaneld", "scoped_ns.cc", ".*Could not open namespace.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Can't retrieve properties for device.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Empty interface name for shill Device \\/device\\/eth\\d.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Failed to obtain service.*GetProperties.*signature.*doesn't exist.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unable to get manager properties.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unknown interface name eth\\d.*", 0},
		{"shill", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},                                                // b/210893108
		{"shill", "dbus_properties_proxy.cc", ".*GetAll failed on org.freedesktop.ModemManager1.*", 0},                            // b/215373366
		{"shill", "device_info.cc", ".*Add link message does not have IFLA_ADDRESS, link: rmnet_ipa0, Technology: ethernet.*", 0}, // b/208654528
		{"shill", "dns_client.cc", ".*No valid DNS server addresses.*", 0},                                                        // b/211000413
		{"shill", "eap_listener.cc", ".*Could not bind socket to interface: No such device.*", 0},                                 // Test lab only
		{"shill", "eap_listener.cc", ".*Could not open EAP listener socket.*", 0},                                                 // Test lab only
		{"shill", "ethernet.cc", ".*cannot disable \\[18\\] tx-tcp-ecn-segmentation.*", 0},                                        // Test lab only
		{"shill", "ethernet.cc", ".*OnSetInterfaceMacResponse received response with error Cannot assign requested address.*", 0}, // Test lab only
		{"shill", "http_request.cc", ".*Failed to start DNS client.*", 0},                                                         // b/211000413
		{"dnsproxyd", "object_proxy.cc", ".*Failed to call method: .*flimflam.Manager.ClearDNSProxyAddresses.*", 0},               // b/239574927
		{"dnsproxyd", "object_proxy.cc", ".*Failed to call method: .*flimflam.Manager.GetProperties.*", 0},                        // b/239574927
		{"dnsproxyd", "client.cc", ".*Unable to get shill Manager properties.*", 0},                                               // b/239574927
		{"shill", "netlink_manager.cc", ".*OnNetlinkMessageError.*Device or resource busy.*", 0},                                  // b/239582086
		{"shill", "network.cc", ".*IP flag write failed:.*", 0},                                                                   // b/243403055
		{"shill", "object_proxy.cc", ".*Failed to call method: fi.w1.wpa_supplicant1.CreateInterface.*", 0},                       // b/215373366
		{"shill", "object_proxy.cc", ".*Failed to call method: fi.w1.wpa_supplicant1.Interface.Scan.*", 0},                        // b/215373366
		{"shill", "object_proxy.cc", ".*Failed to call method: org.chromium.PatchPanel.GetTrafficCounters.*", 0},                  // b/215373366
		{"shill", "object_proxy.cc", ".*Failed to call method: org.chromium.dhcpcd.Release.*", 0},                                 // b/215373366
		{"shill", "object_proxy.cc", ".*Failed to call method: org.freedesktop.DBus.Properties.GetAll.*", 0},                      // b/215373366
		{"shill", "portal_detector.cc", ".*HTTP probe failed to start.*", 0},                                                      // b/213611282
		{"shill", "upstart_proxy.cc", ".*Error.AlreadyStarted Job is already running: shill-event", 0},                            // b/213930243
		// Need to try to get more info about these:
		// {"shill", "unknown", ".*", 0},
		// 'modem in failed state' errors are handled in shill. Because they are DBus errors, suppressing them is difficult:
		{"shill", "utils.cc", ".*AddDBusError.*org.freedesktop.ModemManager1.Error.Core.WrongState, Message=modem in failed state", 0},
		{"shill", "wifi.cc", ".*does not support MAC address randomization.*", 0}, // b/241418700
		{"wpa_supplicant", "", ".*Could not set interface wlan0 flags \\(UP\\): Input\\/output error.*", 0},
		{"wpa_supplicant", "", ".*nl80211: Could not set interface 'wlan0' UP.*", 0},
		{"wpa_supplicant", "", ".*Permission denied.*", 0},
		{"wpa_supplicant", "", ".*wlan0: Failed to initialize driver interface.*", 0},
	}
}
