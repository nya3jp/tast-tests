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
		{"patchpaneld", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"patchpaneld", "manager.cc", ".*Invalid namespace name.*", 0},
		{"patchpaneld", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"patchpaneld", "scoped_ns.cc", ".*Could not open namespace.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unable to get manager properties.*", 0},
		{"shill", "cellular_capability_3gpp.cc", ".*No slot found for.*", 0},
		{"shill", "cellular.cc", ".*StartModem failed.*", 0},
		{"shill", "dbus_method_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"shill", "device_info.cc", ".*Add Link message for.*does not have .*", 0},
		{"shill", "dns_client.cc", ".*No valid DNS server addresses.*", 0},
		{"shill", "http_request.cc", ".*Failed to start DNS client.*", 0},
		{"shill", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"shill", "portal_detector.cc", ".*HTTP probe failed to start.*", 0},
		{"shill", "unknown", ".*", 0},
		{"shill", "utils.cc", ".*AddDBusError.*", 0},
		{"shill", "wifi.cc", ".*does not support MAC address randomization.*", 0},
		{"wpa_supplicant", "", ".*Permission denied.*", 0},
	}
}
