// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import "fmt"

// Formats WiFi configuration for a hostapd instance.
type HostAPConfig struct {
	conf map[string]string
}

func NewHostAPConfig(ssid string) *HostAPConfig {
	return &HostAPConfig{map[string]string{
		"ssid": ssid,

		"logger_syslog":       "-1",
		"logger_syslog_level": "0",
		// default RTS and frag threshold to "off"
		"rts_threshold":   "2347",
		"fragm_threshold": "2346",
		"driver":          "nl80211",

		// TODO(briannorris): parameterize these.
		"hw_mode":     "g",
		"channel":     "6",
		"ieee80211n":  "1",
		"ht_capab":    "[HT40+]",
		"wmm_enabled": "1",
	}}
}

func (ap *HostAPConfig) Format(iface string, ctrlPath string) string {
	ap.conf["interface"] = iface
	ap.conf["ctrl_interface"] = ctrlPath

	s := ""
	for key, val := range ap.conf {
		s = s + fmt.Sprintf("%s=%s\n", key, val)
	}
	return s
}
