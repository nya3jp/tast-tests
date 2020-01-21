// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// WpaModeEnum is the type for specifying WPA modes.
type WpaModeEnum int

// WPA modes.
const (
	WpaPure  WpaModeEnum = 1
	Wpa2Pure WpaModeEnum = 2
	WpaMixed             = WpaPure | Wpa2Pure
)

// Cipher is the type for specifying WPA cipher algorithms.
type Cipher string

// Cipher algorithms.
const (
	CipherTKIP Cipher = "TKIP"
	CipherCCMP Cipher = "CCMP"
)

// FTModeEnum is the type for specifying WPA Fast Transition modes.
type FTModeEnum int

// Fast Transition modes.
const (
	FTModeNone  FTModeEnum = 1
	FTModePure  FTModeEnum = 2
	FTModeMixed            = FTModeNone | FTModePure
)

// WpaConfig is the config used to start hostapd and set properties of DUT.
type WpaConfig struct {
	// Embed BaseConfig so we don't have to re-implement noop methods.
	BaseConfig
	Psk               string
	WpaMode           WpaModeEnum
	WpaCiphers        []Cipher
	Wpa2Ciphers       []Cipher
	WpaPtkRekeyPeriod int
	WpaGtkRekeyPeriod int
	WpaGmkRekeyPeriod int
	UseStrictRekey    bool
	FTMode            FTModeEnum
}

var _ SecurityConfig = (*WpaConfig)(nil)

// GetClass returns security class of WPA network.
func (c *WpaConfig) GetClass() string {
	return "psk"
}

// GetHostapdConfig returns hostapd config of WPA network.
func (c *WpaConfig) GetHostapdConfig() (map[string]string, error) {
	if c.WpaMode == 0 {
		return nil, errors.New("cannot configure WPA unless we know which mode to use")
	}

	if c.WpaMode&WpaPure > 0 && len(c.WpaCiphers) == 0 {
		return nil, errors.New("cannot configure WPA unless we know which ciphers to use")
	}

	if len(c.WpaCiphers) == 0 && len(c.Wpa2Ciphers) == 0 {
		return nil, errors.New("cannot configure WPA2 unless we have some ciphers")
	}

	ret := map[string]string{
		"wpa":          strconv.Itoa(int(c.WpaMode)),
		"wpa_key_mgmt": "WPA-PSK",
	}

	if c.FTMode == FTModePure {
		ret["wpa_key_mgmt"] = "FT-PSK"
	} else if c.FTMode == FTModeMixed {
		ret["wpa_key_mgmt"] = "WPA-PSK FT-PSK"
	}

	if len(c.Psk) == 64 {
		ret["wpa_psk"] = c.Psk
	} else {
		ret["wpa_passphrase"] = c.Psk
	}

	if len(c.WpaCiphers) != 0 {
		ret["wpa_pairwise"] = joinCiphersToString(c.WpaCiphers)
	}
	if len(c.Wpa2Ciphers) != 0 {
		ret["rsn_pairwise"] = joinCiphersToString(c.Wpa2Ciphers)
	}

	if c.WpaPtkRekeyPeriod != 0 {
		ret["wpa_ptk_rekey"] = strconv.Itoa(int(c.WpaPtkRekeyPeriod))
	}
	if c.WpaGtkRekeyPeriod != 0 {
		ret["wpa_group_rekey"] = strconv.Itoa(int(c.WpaGtkRekeyPeriod))
	}
	if c.WpaGmkRekeyPeriod != 0 {
		ret["wpa_gmk_rekey"] = strconv.Itoa(int(c.WpaGmkRekeyPeriod))
	}

	if c.UseStrictRekey {
		ret["wpa_strict_rekey"] = "1"
	}

	return ret, nil
}

// GetShillServiceProperties returns shill properties of WPA network.
func (c *WpaConfig) GetShillServiceProperties() map[string]interface{} {
	ret := map[string]interface{}{
		shill.ServicePropertyPassphrase: c.Psk,
	}
	if c.FTMode&FTModePure > 0 {
		ret[shill.ServicePropertyFtEnabled] = true
	}
	return ret
}

// joinCiphersToString is the utility that concat chiphers with " "
func joinCiphersToString(ciphers []Cipher) (ret string) {
	for _, c := range ciphers {
		if len(ret) == 0 {
			ret = string(c)
		} else {
			ret += " " + string(c)
		}
	}
	return
}
