// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/remote/wificell/dutcfg"
	"fmt"
	"strconv"
)

const routeriface = "br0"
const restartWirelessService = "restart_wireless"
const savedConfigLocation = "/tmp/nvram.cfg"

type Config struct {
	Band               BandEnum
	Ssid               string
	RouterConfigParams []AxRouterConfigParam
	DutConnOptions     []dutcfg.ConnOption
}

// AxRouterConfigParam contains the information to configure a parameter on the ax router.
type AxRouterConfigParam struct {
	Band  BandEnum
	Key   NvramKeyEnum
	Value string
}

// BandEnum is the type for specifying band selection when using the nvram commands.
type BandEnum string

const (
	Wl0 BandEnum = "wl0"
	Wl1 BandEnum = "wl1"
	Wl2 BandEnum = "wl2"
)

type AuthEnum int

const (
	Open AuthEnum = iota
	Psk2
)

type ModeEnum int

const (
	Mode80211ac ModeEnum = iota
	Mode80211ax
)

type ChanBandwidthEnum int

const (
	Bw20 ChanBandwidthEnum = iota
	Bw40
	Bw80
	Bw160
)

type CipherEnum int

const (
	AES CipherEnum = iota
	TKIPAES
)

// NvramKeyEnum is the type for specifying the key to modify when using the nvram commands.
type NvramKeyEnum string

const (
	// KeyAkm refers to the Authentication and Key Management chooses the authentication method (e.g None (""), psk2, etc)
	KeyAkm NvramKeyEnum = "akm"
	// KeyAuthMode Authentication mode (open, psk2)
	KeyAuthMode NvramKeyEnum = "auth_mode_x"
	KeyWpaPsk   NvramKeyEnum = "wpa_psk"
	KeyCrytpo   NvramKeyEnum = "crypto"
	// KeySsid is the router's ssid on the chosen band
	KeySsid NvramKeyEnum = "ssid"
	// KeyChanspec is the band's channel (1,2,3, etc)
	KeyChanspec NvramKeyEnum = "chanspec"
	// KeyClosed is a boolean (0,1) dictating whether the network is hidden (1 is hidden, 0 is open)
	KeyClosed NvramKeyEnum = "closed"
	// KeyRadio is a boolean (0,1) dictating whether the radio is enabled (1 is enabled, 0 is disabled).
	KeyRadio NvramKeyEnum = "radio"
	// KeyHeFeatures is an int (0-3) dictating what level of HE throughput features are supported.
	KeyHeFeatures NvramKeyEnum = "he_features"
	// KeyTxBfBfeCap is a boolean (0,1) dictating whether the radio is capable of being a beamformee
	KeyTxBfBfeCap NvramKeyEnum = "txbf_bfe_cap"
	// KeyTxBfBfeCap is a boolean (0,1) dictating whether the radio is capable of being a beamformer
	KeyTxBfBfrCap NvramKeyEnum = "txbf_bfr_cap"

	KeyBw      NvramKeyEnum = "bw"
	KeyBwCap   NvramKeyEnum = "bw_cap"
	KeyObssCap NvramKeyEnum = "obss_coex"
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

func Ssid(ssid string) Option {
	return func(c *Config) {
		c.Ssid = ssid
		c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{c.Band, KeySsid, ssid})
	}
}

func Hidden(hidden bool) Option {
	return func(c *Config) {
		var res int
		if hidden {
			res = 1
		} else {
			res = 0
		}
		c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{c.Band, KeyClosed, strconv.Itoa(res)})
		c.DutConnOptions = append(c.DutConnOptions, dutcfg.ConnHidden(true))
	}
}

func Channel(channel int) Option {
	return func(c *Config) {
		var suffix string
		switch c.Band {
		case Wl0:
			suffix = "l"
		default:
			suffix = "/80"
		}
		c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{c.Band, KeyChanspec, fmt.Sprintf("%d%s", channel, suffix)})
	}
}

func AuthType(auth AuthEnum) Option {
	return func(c *Config) {
		switch auth {
		case Open:
			c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{c.Band, KeyAkm, ""}, AxRouterConfigParam{c.Band, KeyAuthMode, "open"})
		}
	}
}

func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		switch mode {
		case Mode80211ac:
			for _, band := range []BandEnum{Wl0, Wl1, Wl2} {
				c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{band, KeyHeFeatures, "0"}, AxRouterConfigParam{band, KeyTxBfBfeCap, "1"}, AxRouterConfigParam{band, KeyTxBfBfrCap, "1"})
			}
		case Mode80211ax:
			for _, band := range []BandEnum{Wl0, Wl1, Wl2} {
				c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{band, KeyHeFeatures, "3"}, AxRouterConfigParam{band, KeyTxBfBfeCap, "5"}, AxRouterConfigParam{band, KeyTxBfBfrCap, "5"})
			}
		}
	}
}

func ChanBandwidth(bw ChanBandwidthEnum) Option {
	return func(c *Config) {
		var bandw, bwCap, obssCap int
		switch bw {
		case Bw20:
			bandw = 1
			bwCap = 1
			obssCap = 0
		case Bw40:
			bandw = 2
			bwCap = 3
			obssCap = 0
		case Bw80:
			bandw = 3
			bwCap = 7
			obssCap = 0
		case Bw160:
			bandw = 5
			bwCap = 15
			obssCap = 1
		}
		c.RouterConfigParams = append(c.RouterConfigParams, AxRouterConfigParam{c.Band, KeyBw, strconv.Itoa(bandw)}, AxRouterConfigParam{c.Band, KeyBwCap, strconv.Itoa(bwCap)}, AxRouterConfigParam{c.Band, KeyObssCap, strconv.Itoa(obssCap)})
	}
}
