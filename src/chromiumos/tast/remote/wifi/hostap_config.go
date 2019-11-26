// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// Port from Brian's PoC crrev.com/c/1733740

// Mode is the type for specifying hostap mode.
type Mode string

// Mode enums.
const (
	Mode80211a      Mode = "a"
	Mode80211b      Mode = "b"
	Mode80211g      Mode = "g"
	Mode80211nMixed Mode = "n-mixed"
	Mode80211nPure  Mode = "n-only"
)

// HTCap is the type for specifying HT capabilities.
type HTCap int

// HTCap enums, use bitmask for ease of checking existance.
const (
	// HTCapHT20 is the default when none of HT40* specified.
	HTCapHT40Minus HTCap = 1 << iota // "[HT40-]"
	HTCapHT40Plus                    // "[HT40+]"
	HTCapSGI20                       // "[SHORT-GI-20]"
	HTCapSGI40                       // "[SHORT-GI-40]"
)

// TODO: As we have options, maybe it's better to give hostap a independent package,
// and the names can be simpler.

// HostAPOption is the type used to specify options of HostAPConfig.
type HostAPOption func(*HostAPConfig)

// HostAPMode returns a HostAPOption which sets hw_mode in hostapd config.
func HostAPMode(mode Mode) HostAPOption {
	return func(c *HostAPConfig) {
		c.Mode = mode
	}
}

// HostAPChannel returns a HostAPOption which sets channel in hostapd config.
func HostAPChannel(ch int) HostAPOption {
	return func(c *HostAPConfig) {
		c.Channel = ch
	}
}

// HostAPHTCap returns a HostAPOption which sets ht capability in hostapd config.
func HostAPHTCap(ca HTCap) HostAPOption {
	return func(c *HostAPConfig) {
		c.HTCaps |= ca
	}
}

// NewHostAPConfig creates a HostAPConfig with given options.
func NewHostAPConfig(ssid string, ops ...HostAPOption) *HostAPConfig {
	// Default config.
	conf := &HostAPConfig{
		Ssid:    ssid,
		Mode:    Mode80211g,
		Channel: 1,
	}
	for _, op := range ops {
		op(conf)
	}
	return conf
}

// HostAPConfig is the config used to start hostapd on router.
type HostAPConfig struct {
	Ssid    string
	Mode    Mode
	Channel int
	HTCaps  HTCap
}

// Format the config into hostapd.conf format.
func (c *HostAPConfig) Format(iface, ctrlPath string) (string, error) {
	var builder strings.Builder
	configure := func(k, v string) {
		builder.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}

	configure("logger_syslog", "-1")
	configure("logger_syslog_level", "0")
	// Default RTS and frag threshold to "off".
	configure("rts_threshold", "2347")
	configure("fragm_threshold", "2346")
	configure("driver", "nl80211")

	// Configurable.
	configure("ctrl_interface", ctrlPath)
	configure("ssid", c.Ssid)
	configure("interface", iface)
	configure("channel", strconv.Itoa(c.Channel))

	hwMode, err := c.hwMode()
	if err != nil {
		return "", err
	}
	configure("hw_mode", hwMode)

	if c.is80211n() {
		configure("ieee80211n", "1")
		htCaps, err := c.htCapsString()
		if err != nil {
			return "", err
		}
		configure("ht_capab", htCaps)
	}
	if c.HTCaps != 0 {
		configure("wmm_enabled", "1")
	}

	return builder.String(), nil
}

// utilities for HostAPConfig to generate config map.

func (c *HostAPConfig) is80211n() bool {
	return c.Mode == Mode80211nMixed || c.Mode == Mode80211nPure
}

func (c *HostAPConfig) hwMode() (string, error) {
	if !c.is80211n() {
		return string(c.Mode), nil
	}
	f, err := channelToFrequency(c.Channel)
	if err != nil {
		return "", nil
	}
	if f > 5000 {
		return string(Mode80211a), nil
	}
	return string(Mode80211g), nil
}

func channelIn(ch int, list []int) bool {
	for _, c := range list {
		if c == ch {
			return true
		}
	}
	return false
}

var ht40MinusChannels = []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 40, 48, 56, 64, 104, 112, 120, 128, 136, 144, 153, 161}
var ht40PlusChannels = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 36, 44, 52, 60, 100, 108, 116, 124, 132, 140, 149, 157}

func (c *HostAPConfig) htCapsString() (string, error) {
	var caps []string
	if c.HTCaps&HTCapHT40Minus > 0 {
		if !channelIn(c.Channel, ht40MinusChannels) {
			return "", errors.Errorf("channel %d does not support HT40-", c.Channel)
		}
		caps = append(caps, "[HT40-]")
	}
	if c.HTCaps&HTCapHT40Plus > 0 {
		if !channelIn(c.Channel, ht40PlusChannels) {
			return "", errors.Errorf("channel %d does not support HT40+", c.Channel)
		}
		caps = append(caps, "[HT40+]")
	}
	if c.HTCaps&HTCapSGI20 > 0 {
		caps = append(caps, "[SHORT-GI-20]")
	}
	if c.HTCaps&HTCapSGI40 > 0 {
		caps = append(caps, "[SHORT-GI-40]")
	}
	return strings.Join(caps, ""), nil
}
