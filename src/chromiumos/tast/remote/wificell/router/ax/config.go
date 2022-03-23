// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ax

import (
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/dutcfg"
)

// RouterIface is the default interface used by the ASUS router.
const RouterIface = "br0"

// RestartWirelessService is the binary name used to restart the wireless server by the ASUS router.
const RestartWirelessService = "restart_wireless"

// SavedConfigLocation is the path to save the temporary NVRAM config.
const SavedConfigLocation = "/tmp/nvram.cfg"

// DeviceType is an enum indicating what model an AxRouter is.
type DeviceType int

const (
	// GtAx11000 is for the GT-Ax11000 device,
	GtAx11000 DeviceType = iota
	// GtAxe11000 is for the GT-Axe11000 device (6e).
	GtAxe11000
	// Ax6100 is for the Ax6100 device.
	Ax6100
	// Unknown means DeviceType will be resolved based on host.
	Unknown
)

var deviceTypeValueToString = map[DeviceType]string{
	GtAx11000:  "GtAx11000",
	GtAxe11000: "GtAxe11000",
	Ax6100:     "Ax6100",
	Unknown:    "Unknown",
}

// String returns a human-readable string describing the DeviceType.
func (dt DeviceType) String() string {
	typeStr, ok := deviceTypeValueToString[dt]
	if !ok {
		return string(rune(dt))
	}
	return typeStr
}

// DeviceTypeFromString parses deviceType for its corresponding DeviceType.
func DeviceTypeFromString(deviceType string) (DeviceType, error) {
	for dt, dtStr := range deviceTypeValueToString {
		if strings.EqualFold(deviceType, dtStr) {
			return dt, nil
		}
	}
	return -1, errors.Errorf("invalid AX DeviceType %q", deviceType)
}

// Config stores the necessary information for an AX test to run.
type Config struct {
	Type               DeviceType
	Band               RadioEnum
	SSID               string
	NVRAMOut           *string
	RouterRecoveryMap  map[string]ConfigParam
	RouterConfigParams []ConfigParam
	DUTConnOptions     []dutcfg.ConnOption
}

// ConfigParam contains the information to configure a parameter on the ax router.
type ConfigParam struct {
	Band  RadioEnum
	Key   NVRAMKeyEnum
	Value string
}

// BandEnum is the type for specifying the radio's transmission frequency desired from the test.
type BandEnum int

const (
	// Ghz2 corresponds to the 2ghz band on the router.
	Ghz2 BandEnum = iota
	// Ghz5 corresponds to the 5ghz band on the router.
	Ghz5
	// Ghz6 corresponds to the 6ghz band on the router.
	Ghz6
)

// RadioEnum is the type for specifying band selection when using the NVRAM commands.
type RadioEnum string

const (
	// Wl0 is the first radio on the router.
	Wl0 RadioEnum = "wl0"
	// Wl1 is the second radio on the router.
	Wl1 RadioEnum = "wl1"
	// Wl2 is the third radio on the router.
	Wl2 RadioEnum = "wl2"
	// WlInvalid is an invalid radio on the router.
	WlInvalid RadioEnum = "INVALID"
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
	// Bw20 is 20Mhz.
	Bw20 ChanBandwidthEnum = iota
	// Bw40 is 40Mhz.
	Bw40
	// Bw80 is 80Mhz.
	Bw80
	// Bw160 is 160Mhz.
	Bw160
)

// CipherEnum selects what supported ciphers are for WPA authentication.
type CipherEnum int

const (
	// AES supports the AES symmetrical cipher.
	AES CipherEnum = iota
	// TKIPAES supports the mixed TKIP and AES ciphers.
	TKIPAES
)

// NVRAMKeyEnum is the type for specifying the key to modify when using the NVRAM commands.
type NVRAMKeyEnum string

const (
	// KeyAKM refers to the Authentication and Key Management chooses the authentication method (e.g None (""), psk2, etc).
	KeyAKM NVRAMKeyEnum = "akm"
	// KeyAuthMode Authentication mode (open, psk2).
	KeyAuthMode NVRAMKeyEnum = "auth_mode_x"
	// KeyWPAPSK is the preshared key for wpa authentication.
	KeyWPAPSK NVRAMKeyEnum = "wpa_psk"
	// KeyCrypto is the supported ciphers for wpa authentication.
	KeyCrypto NVRAMKeyEnum = "crypto"
	// KeySSID is the router's ssid on the chosen band.
	KeySSID NVRAMKeyEnum = "ssid"
	// KeyChanspec is the band's channel (1,2,3, etc).
	KeyChanspec NVRAMKeyEnum = "chanspec"
	// KeyClosed is a boolean (0,1) dictating whether the network is hidden (1 is hidden, 0 is open).
	KeyClosed NVRAMKeyEnum = "closed"
	// KeyRadio is a boolean (0,1) dictating whether the radio is enabled (1 is enabled, 0 is disabled).
	KeyRadio NVRAMKeyEnum = "radio"
	// KeyHeFeatures is an int (0-3) dictating what level of HE throughput features are supported.
	KeyHeFeatures NVRAMKeyEnum = "he_features"
	// KeyTxBfBfeCap is a boolean (0,1) dictating whether the radio is capable of being a beamformee.
	KeyTxBfBfeCap NVRAMKeyEnum = "txbf_bfe_cap"
	// KeyTxBfBfrCap is a boolean (0,1) dictating whether the radio is capable of being a beamformer.
	KeyTxBfBfrCap NVRAMKeyEnum = "txbf_bfr_cap"
	// KeyBw is an int dictating the router's bandwidth value.
	KeyBw NVRAMKeyEnum = "bw"
	// KeyBwCap is an int dictating the router's bandwidth capabilities.
	KeyBwCap NVRAMKeyEnum = "bw_cap"
	// KeyOBSSCap is a boolean (0,1) dictating the router's 160Mhz capabilities on GT0AX11000 devices.
	KeyOBSSCap NVRAMKeyEnum = "obss_coex"
	// KeyBw160 is a boolean (0,1) dictating the router's 160Mhz capabilities on AX6100 devices only.
	KeyBw160 NVRAMKeyEnum = "bw_160"
	// Key11Ax is a boolean (0,1) dictating the router's AX capabilities for Ax6100 only
	Key11Ax NVRAMKeyEnum = "11ax"
	// KeyBSSOpModeCapReqd is an int dictating the required capabilities required for BSS Operation (for Ax6100 only).
	KeyBSSOpModeCapReqd NVRAMKeyEnum = "bss_opmode_cap_reqd"
	// KeyNModeX is an int. Required for AX operation on Ax6100 devices. (0,9).
	KeyNModeX NVRAMKeyEnum = "nmode_x"
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// SSID sets the ssid of the AP.
func SSID(ssid string) Option {
	return func(c *Config) {
		c.SSID = ssid
		c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeySSID, ssid})
	}
}

// BandToRadio converts the desired Band to the appropriate radio on the Ax Router.
func BandToRadio(axtype DeviceType, r BandEnum) RadioEnum {
	switch r {
	case Ghz2:
		return Wl0
	case Ghz5:
		if axtype == GtAxe11000 {
			return Wl1
		}
		return Wl2
	case Ghz6:
		if axtype == GtAxe11000 {
			return Wl2
		}
		return WlInvalid
	}
	return WlInvalid
}

// Hidden sets whether the AP is hidden.
func Hidden(hidden bool) Option {
	return func(c *Config) {
		var res int
		if hidden {
			res = 1
		} else {
			res = 0
		}
		c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyClosed, strconv.Itoa(res)})
		c.DUTConnOptions = append(c.DUTConnOptions, dutcfg.ConnHidden(hidden))
	}
}

// Radio sets whether the Radio is enabled.
func Radio(enabled bool) Option {
	return func(c *Config) {
		if enabled {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyRadio, "1"})
		} else {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyRadio, "0"})
		}
	}
}

// Mode sets the AP protocol of the router.
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		switch mode {
		case Mode80211ac:
			if c.Type == GtAx11000 || c.Type == GtAxe11000 {
				for _, band := range []RadioEnum{Wl0, Wl1, Wl2} {
					c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{band, KeyHeFeatures, "0"}, ConfigParam{band, KeyTxBfBfeCap, "1"}, ConfigParam{band, KeyTxBfBfrCap, "1"})
				}
			} else if c.Type == Ax6100 {
				c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{Wl2, Key11Ax, "0"}, ConfigParam{Wl2, KeyBSSOpModeCapReqd, "0"}, ConfigParam{Wl2, KeyNModeX, "0"})
			}

		case Mode80211ax:
			if c.Type == GtAx11000 || c.Type == GtAxe11000 {
				for _, band := range []RadioEnum{Wl0, Wl1, Wl2} {
					c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{band, KeyHeFeatures, "3"}, ConfigParam{band, KeyTxBfBfeCap, "5"}, ConfigParam{band, KeyTxBfBfrCap, "5"})
				}
			} else if c.Type == Ax6100 {
				c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{Wl2, Key11Ax, "1"}, ConfigParam{Wl2, KeyBSSOpModeCapReqd, "4"}, ConfigParam{Wl2, KeyNModeX, "9"})
			}
		}
	}
}

// ChanBandwidth sets the APs supported channel width.
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
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyBw, strconv.Itoa(bandw)}, ConfigParam{c.Band, KeyBwCap, strconv.Itoa(bwCap)}, ConfigParam{c.Band, KeyOBSSCap, strconv.Itoa(bw160)})
		} else if c.Type == Ax6100 {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyBw, strconv.Itoa(bandw)}, ConfigParam{c.Band, KeyBwCap, strconv.Itoa(bwCap)}, ConfigParam{c.Band, KeyBw160, strconv.Itoa(bw160)})
		}

		// GtAxe11000 applies a channel prefix, "6g" if it is transmitting on the 6ghz band.
		if c.Type == GtAxe11000 && c.Band == Wl2 {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyChanspec, fmt.Sprintf("6g%d%s", ch, suffix)})
		} else {
			c.RouterConfigParams = append(c.RouterConfigParams, ConfigParam{c.Band, KeyChanspec, fmt.Sprintf("%d%s", ch, suffix)})
		}
	}
}
