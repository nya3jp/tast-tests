// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package axrouter

import (
	"fmt"
	"strconv"

	"chromiumos/tast/remote/wificell/dutcfg"
)

// Routeriface is the default interface used by the ASUS router
const Routeriface = "br0"

// RestartWirelessService is the binary name used to restart the wireless server by the ASUS router.
const RestartWirelessService = "restart_wireless"

// SavedConfigLocation is the path to save the temporary nvram config.
const SavedConfigLocation = "/tmp/nvram.cfg"

// AxType is an enum indicating what model an AxRouter is.
type AxType int

const (
	// GtAx11000 is for the GT-Ax11000 device,
	GtAx11000 AxType = iota
	// Ax6100 is for the Ax6100 device.
	Ax6100
	// Invalid is the default AxType.
	Invalid
)

// Config stores the necessary information for an AX test to run.
type Config struct {
	Type               AxType
	Band               BandEnum
	Ssid               string
	NvramOut           *string
	RouterRecoveryMap  map[string]ConfigParam
	RouterConfigParams []ConfigParam
	DutConnOptions     []dutcfg.ConnOption
}

// ConfigParam contains the information to configure a parameter on the ax router.
type ConfigParam struct {
	Band  BandEnum
	Key   NvramKeyEnum
	Value string
}

// BandEnum is the type for specifying band selection when using the nvram commands.
type BandEnum string

const (
	// Wl0 is the 2.4Ghz band on the router.
	Wl0 BandEnum = "wl0"
	// Wl1 is the first 5Ghz band on the router.
	Wl1 BandEnum = "wl1"
	// Wl2 is the second 5Ghz band (gaming) on the router.
	Wl2 BandEnum = "wl2"
)

// ModeEnum selects which WiFi protocol should be used for the AP.
type ModeEnum int

const (
	// Mode80211ac tells the router to support 802.11ac.
	Mode80211ac ModeEnum = iota
	// Mode80211ax tells the router to support 802.11ax.
	Mode80211ax
)

// ChanBandwidthEnum selects what the channel bandwidth of the AP should be.
type ChanBandwidthEnum int

const (
	// Bw20 is 20Mhz
	Bw20 ChanBandwidthEnum = iota
	// Bw40 is 40Mhz
	Bw40
	// Bw80 is 80Mhz
	Bw80
	// Bw160 is 160Mhz
	Bw160
)

// CipherEnum selects what supported ciphers are for WPA authentication
type CipherEnum int

const (
	// AES supports the AES symmetrical cipher
	AES CipherEnum = iota
	// TKIPAES supports the mixed TKIP and AES ciphers.
	TKIPAES
)

// NvramKeyEnum is the type for specifying the key to modify when using the nvram commands.
type NvramKeyEnum string

const (
	// KeyAkm refers to the Authentication and Key Management chooses the authentication method (e.g None (""), psk2, etc)
	KeyAkm NvramKeyEnum = "akm"
	// KeyAuthMode Authentication mode (open, psk2)
	KeyAuthMode NvramKeyEnum = "auth_mode_x"
	// KeyWpaPsk is the preshared key for wpa authentication.
	KeyWpaPsk NvramKeyEnum = "wpa_psk"
	// KeyCrypto is the supported ciphers for wpa authentication.
	KeyCrypto NvramKeyEnum = "crypto"
	// KeySsid is the router's ssid on the chosen band
	KeySsid NvramKeyEnum = "ssid"
	// KeyChanspec is the band's channel (1,2,3, etc)
	KeyChanspec NvramKeyEnum = "chanspec"
	// KeyClosed is a boolean (0,1) dictating whether the network is hidden (1 is hidden, 0 is open)
	KeyClosed NvramKeyEnum = "closed"
	// KeyRadio is a boolean (0,1) dictating whether the radio is enabled (1 is enabled, 0 is disabled)
	KeyRadio NvramKeyEnum = "radio"
	// KeyHeFeatures is an int (0-3) dictating what level of HE throughput features are supported.
	KeyHeFeatures NvramKeyEnum = "he_features"
	// KeyTxBfBfeCap is a boolean (0,1) dictating whether the radio is capable of being a beamformee
	KeyTxBfBfeCap NvramKeyEnum = "txbf_bfe_cap"
	// KeyTxBfBfrCap is a boolean (0,1) dictating whether the radio is capable of being a beamformer
	KeyTxBfBfrCap NvramKeyEnum = "txbf_bfr_cap"
	// KeyBw is an int dictating the router's bandwidth value
	KeyBw NvramKeyEnum = "bw"
	// KeyBwCap is an int dictating the router's bandwidth capabilities
	KeyBwCap NvramKeyEnum = "bw_cap"
	// KeyObssCap is a boolean (0,1) dictating the router's 160Mhz capabilities on GT0AX11000 devices
	KeyObssCap NvramKeyEnum = "obss_coex"
	// KeyBw160 is a boolean (0,1) dictating the router's 160Mhz capabilities on AX6100 devices only
	KeyBw160 NvramKeyEnum = "bw_160"
	// Key11Ax is a boolean (0,1) dictating the router's AX capabilities for Ax6100 only
	Key11Ax NvramKeyEnum = "11ax"
	// KeyBssOpModeCapReqd is an int dictating the required capabilities required for BSS Operation (for Ax6100 only)
	KeyBssOpModeCapReqd NvramKeyEnum = "bss_opmode_cap_reqd"
	// KeyNModeX is an int. Required for AX operation on Ax6100 devices. (0,9)
	KeyNModeX NvramKeyEnum = "nmode_x"
)

// Option is the function signature used to specify options of Config
type Option func(*Config)

// Ssid sets the ssid of the AP
func Ssid(ssid string) Option {
	return func(c *Config) {
		c.Ssid = ssid
		c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeySsid, ssid})
	}
}

// Hidden sets whether the AP is hidden
func Hidden(hidden bool) Option {
	return func(c *Config) {
		var res int
		if hidden {
			res = 1
		} else {
			res = 0
		}
		c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyClosed, strconv.Itoa(res)})
		c.DutConnOptions = append(c.DutConnOptions, dutcfg.ConnHidden(true))
	}
}

// Radio sets whether the Radio is enabled
func Radio(enabled bool) Option {
	return func(c *Config) {
		if enabled {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyRadio, "1"})
		} else {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyRadio, "0"})
		}
	}
}

// Mode sets the AP protocol of the router
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		switch mode {
		case Mode80211ac:
			if c.Type == GtAx11000 {
				for _, band := range []BandEnum{Wl0, Wl1, Wl2} {
					c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{band, KeyHeFeatures, "0"}, ConfigParam{band, KeyTxBfBfeCap, "1"}, ConfigParam{band, KeyTxBfBfrCap, "1"})
				}
			} else if c.Type == Ax6100 {
				c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{Wl2, Key11Ax, "0"}, ConfigParam{Wl2, KeyBssOpModeCapReqd, "0"}, ConfigParam{Wl2, KeyNModeX, "0"})
			}

		case Mode80211ax:
			if c.Type == GtAx11000 {
				for _, band := range []BandEnum{Wl0, Wl1, Wl2} {
					c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{band, KeyHeFeatures, "3"}, ConfigParam{band, KeyTxBfBfeCap, "5"}, ConfigParam{band, KeyTxBfBfrCap, "5"})
				}
			} else if c.Type == Ax6100 {
				c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{Wl2, Key11Ax, "1"}, ConfigParam{Wl2, KeyBssOpModeCapReqd, "4"}, ConfigParam{Wl2, KeyNModeX, "9"})
			}
		}
	}
}

// ChanBandwidth sets the APs supported channel width
func ChanBandwidth(ch int, bw ChanBandwidthEnum) Option {
	return func(c *Config) {
		// var bandw, bwCap, obssCap int
		var bandw, bwCap, bw160 int
		var suffix string
		switch bw {
		case Bw20:
			bandw = 1
			bwCap = 1
			bw160 = 0
			suffix = ""
		case Bw40:
			bandw = 2
			bwCap = 3
			bw160 = 0
			suffix = "l"
		case Bw80:
			bandw = 3
			bwCap = 7
			bw160 = 0
			suffix = "/80"
		case Bw160:
			bandw = 5
			bwCap = 15
			bw160 = 1
			suffix = "/160"
		}
		if c.Type == GtAx11000 {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyBw, strconv.Itoa(bandw)}, ConfigParam{c.Band, KeyBwCap, strconv.Itoa(bwCap)}, ConfigParam{c.Band, KeyObssCap, strconv.Itoa(bw160)})
		} else if c.Type == Ax6100 {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyBw, strconv.Itoa(bandw)}, ConfigParam{c.Band, KeyBwCap, strconv.Itoa(bwCap)}, ConfigParam{c.Band, KeyBw160, strconv.Itoa(bw160)})
		}
		c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyChanspec, fmt.Sprintf("%d%s", ch, suffix)})
	}
}
