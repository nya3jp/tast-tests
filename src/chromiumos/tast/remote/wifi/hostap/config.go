// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostap

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wifi/hostap/secconf"
	"chromiumos/tast/remote/wifi/utils"
	"chromiumos/tast/testing"
)

// Ported from Brian's draft crrev.com/c/1733740.

// ModeEnum is the type for specifying hostap mode.
type ModeEnum string

// Mode enums.
const (
	Mode80211a       ModeEnum = "a"
	Mode80211b       ModeEnum = "b"
	Mode80211g       ModeEnum = "g"
	Mode80211nMixed  ModeEnum = "n-mixed"
	Mode80211nPure   ModeEnum = "n-only"
	Mode80211acMixed ModeEnum = "ac-mixed"
	Mode80211acPure  ModeEnum = "ac-only"
)

// HTCap is the type for specifying HT capabilities in hostapd config (ht_capab=).
type HTCap int

// HTCap enums, use bitmask for ease of checking existance.
const (
	// HTCapHT20 is the default when none of HT40* specified.
	HTCapHT40       HTCap = 1 << iota // auto-detect supported "[HT40-]" or "[HT40+]"
	HTCapHT40Minus                    // "[HT40-]"
	HTCapHT40Plus                     // "[HT40+]"
	HTCapSGI20                        // "[SHORT-GI-20]"
	HTCapSGI40                        // "[SHORT-GI-40]"
	HTCapGreenfield                   // "[GF]"
)

// VHTCap is the type for specifying VHT capabilities in hostapd config (vht_capab=).
type VHTCap string

// Each capability can be simply mapped to a string.
const (
	VHTCapVHT160             VHTCap = "[VHT160]"
	VHTCapVHT16080Plus80     VHTCap = "[VHT160-80PLUS80]"
	VHTCapRXLDPC             VHTCap = "[RXLDPC]"
	VHTCapSGI80              VHTCap = "[SHORT-GI-80]"
	VHTCapSGI160             VHTCap = "[SHORT-GI-160]"
	VHTCapTxSTBC2BY1         VHTCap = "[TX-STBC-2BY1]"
	VHTCapRxSTBC1            VHTCap = "[RX-STBC-1]"
	VHTCapRxSTBC12           VHTCap = "[RX-STBC-12]"
	VHTCapRxSTBC123          VHTCap = "[RX-STBC-123]"
	VHTCapRxSTBC1234         VHTCap = "[RX-STBC-1234]"
	VHTCapSUBeamformer       VHTCap = "[SU-BEAMFORMER]"
	VHTCapSUBeamformee       VHTCap = "[SU-BEAMFORMEE]"
	VHTCapBFAntenna2         VHTCap = "[BF-ANTENNA-2]"
	VHTCapSoundingDimension2 VHTCap = "[SOUNDING-DIMENSION-2]"
	VHTCapMUBeamformer       VHTCap = "[MU-BEAMFORMER]"
	VHTCapMUBeamformee       VHTCap = "[MU-BEAMFORMEE]"
	VHTCapVHTTXOPPS          VHTCap = "[VHT-TXOP-PS]"
	VHTCapHTCVHT             VHTCap = "[HTC-VHT]"
	VHTCapMaxAMPDULenExp0    VHTCap = "[MAX-A-MPDU-LEN-EXP0]"
	VHTCapMaxAMPDULenExp1    VHTCap = "[MAX-A-MPDU-LEN-EXP1]"
	VHTCapMaxAMPDULenExp2    VHTCap = "[MAX-A-MPDU-LEN-EXP2]"
	VHTCapMaxAMPDULenExp3    VHTCap = "[MAX-A-MPDU-LEN-EXP3]"
	VHTCapMaxAMPDULenExp4    VHTCap = "[MAX-A-MPDU-LEN-EXP4]"
	VHTCapMaxAMPDULenExp5    VHTCap = "[MAX-A-MPDU-LEN-EXP5]"
	VHTCapMaxAMPDULenExp6    VHTCap = "[MAX-A-MPDU-LEN-EXP6]"
	VHTCapMaxAMPDULenExp7    VHTCap = "[MAX-A-MPDU-LEN-EXP7]"
	VHTCapVHTLinkADAPT2      VHTCap = "[VHT-LINK-ADAPT2]"
	VHTCapVHTLinkADAPT3      VHTCap = "[VHT-LINK-ADAPT3]"
	VHTCapRxAntennaPattern   VHTCap = "[RX-ANTENNA-PATTERN]"
	VHTCapTxAntennaPattern   VHTCap = "[TX-ANTENNA-PATTERN]"
)

// VHTChWidthEnum is the type for specifying operating channel width in hostapd config (vht_oper_chwidth=).
type VHTChWidthEnum int

// VHTChWidth enums.
const (
	// VHTChWidth40 is the default value when none of VHTChWidth* specified.
	VHTChWidth40 VHTChWidthEnum = iota
	VHTChWidth80
	VHTChWidth160
	VHTChWidth80Plus80
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// SSID return an Option which sets ssid in hostapd config.
func SSID(ssid string) Option {
	return func(c *Config) {
		c.Ssid = ssid
	}
}

// Mode returns an Option which sets mode in hostapd config.
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		c.Mode = mode
	}
}

// Channel returns an Option which sets channel in hostapd config.
func Channel(ch int) Option {
	return func(c *Config) {
		c.Channel = ch
	}
}

// HTCaps returns an Option which sets ht capabilities in hostapd config.
func HTCaps(caps ...HTCap) Option {
	return func(c *Config) {
		for _, ca := range caps {
			c.HTCaps |= ca
		}
	}
}

// VHTCaps returns an Option which sets vht capabilities in hostapd config.
func VHTCaps(caps ...VHTCap) Option {
	return func(c *Config) {
		c.VHTCaps = append(c.VHTCaps, caps...)
	}
}

// VHTCenterChannel returns an Option which sets vht center channel in hostapd config.
func VHTCenterChannel(ch int) Option {
	return func(c *Config) {
		c.VHTCenterChannel = ch
	}
}

// VHTChWidth returns an Option which sets vht operating channel width in hostapd config.
func VHTChWidth(chw VHTChWidthEnum) Option {
	return func(c *Config) {
		c.VHTChWidth = chw
	}
}

// Hidden returns an Option which sets it is hidden network or not in hostapd config.
func Hidden(h bool) Option {
	return func(c *Config) {
		c.Hidden = h
	}
}

// SecurityConfig returns an Option which sets the config of security protocol in hostapd config.
func SecurityConfig(security secconf.SecurityConfig) Option {
	return func(c *Config) {
		c.SecurityConfig = security
	}
}

// NewConfig creates a Config with given options.
func NewConfig(ops ...Option) *Config {
	// Default config.
	conf := &Config{
		Ssid:           RandomSSID("TAST_TEST_"),
		Mode:           Mode80211g,
		Channel:        1,
		SecurityConfig: &secconf.BaseConfig{},
	}
	for _, op := range ops {
		op(conf)
	}
	return conf
}

// Config is the config used to start hostapd on router.
type Config struct {
	Ssid             string
	Mode             ModeEnum
	Channel          int
	HTCaps           HTCap
	VHTCaps          []VHTCap
	VHTCenterChannel int
	VHTChWidth       VHTChWidthEnum
	Hidden           bool
	SecurityConfig   secconf.SecurityConfig
}

// Format the config into hostapd.conf format.
// iface is the interface for the hostapd to run. ctrlPath is the control
// file path for hostapd to communicate with hostapd_cli.
func (c *Config) Format(ctx context.Context, iface, ctrlPath string) (string, error) {
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

	if c.is80211n() || c.is80211ac() {
		configure("ieee80211n", "1")
		htCaps, err := c.htCapsString(ctx)
		if err != nil {
			return "", err
		}
		configure("ht_capab", htCaps)
		if c.Mode == Mode80211nPure {
			configure("require_ht", "1")
		}
	}
	if c.is80211ac() {
		configure("ieee80211ac", "1")
		chw, err := c.vhtOperChWidthString()
		if err != nil {
			return "", err
		}
		configure("vht_oper_chwidth", chw)
		// If not set, ignore this field and use hostapd's default value.
		if c.VHTCenterChannel != 0 {
			configure("vht_oper_centr_freq_seg0_idx", strconv.Itoa(c.VHTCenterChannel))
		}
		configure("vht_capab", c.vhtCapsString())
		if c.Mode == Mode80211acPure {
			configure("require_vht", "1")
		}
	}
	if c.HTCaps != 0 {
		configure("wmm_enabled", "1")
	}
	if c.Hidden {
		configure("ignore_broadcast_ssid", "1")
	}

	security, err := c.SecurityConfig.GetHostapdConfig()
	if err != nil {
		return "", err
	}
	for k, v := range security {
		configure(k, v)
	}

	return builder.String(), nil
}

// utilities for HostAPConfig to generate config map.

func (c *Config) is80211n() bool {
	return c.Mode == Mode80211nMixed || c.Mode == Mode80211nPure
}

func (c *Config) is80211ac() bool {
	return c.Mode == Mode80211acMixed || c.Mode == Mode80211acPure
}

func (c *Config) hwMode() (string, error) {
	if c.Mode == Mode80211a || c.Mode == Mode80211b || c.Mode == Mode80211g {
		return string(c.Mode), nil
	}
	if c.is80211n() || c.is80211ac() {
		f, err := utils.ChannelToFrequency(c.Channel)
		if err != nil {
			return "", err
		}
		if f > 5000 {
			return string(Mode80211a), nil
		}
		return string(Mode80211g), nil
	}
	return "", errors.Errorf("invalid mode %s", string(c.Mode))
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

func (c *Config) htCapsString(ctx context.Context) (string, error) {
	var caps []string
	if c.HTCaps&(HTCapHT40|HTCapHT40Minus) > 0 && channelIn(c.Channel, ht40MinusChannels) {
		caps = append(caps, "[HT40-]")
	}
	if c.HTCaps&(HTCapHT40|HTCapHT40Plus) > 0 && channelIn(c.Channel, ht40PlusChannels) {
		caps = append(caps, "[HT40+]")
	}
	if c.HTCaps&HTCapSGI20 > 0 {
		caps = append(caps, "[SHORT-GI-20]")
	}
	if c.HTCaps&HTCapSGI40 > 0 {
		caps = append(caps, "[SHORT-GI-40]")
	}
	if c.HTCaps&HTCapGreenfield > 0 {
		testing.ContextLog(ctx, "Greenfield flag is ignored")
	}
	return strings.Join(caps, ""), nil
}

func (c *Config) vhtCapsString() (ret string) {
	for _, v := range c.VHTCaps {
		ret += string(v)
	}
	return
}

func (c *Config) vhtOperChWidthString() (string, error) {
	switch c.VHTChWidth {
	case VHTChWidth40:
		return "0", nil
	case VHTChWidth80:
		return "1", nil
	case VHTChWidth160:
		return "2", nil
	case VHTChWidth80Plus80:
		return "3", nil
	default:
		return "", errors.Errorf("invalid vht_oper_chwidth %d", int(c.VHTChWidth))
	}
}

// RandomSSID returns a random SSID of length 30 and given prefix.
func RandomSSID(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Generate 30-char SSID, including prefix
	n := 30 - len(prefix)
	s := make([]byte, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return prefix + string(s)
}
