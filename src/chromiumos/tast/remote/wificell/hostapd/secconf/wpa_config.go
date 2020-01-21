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

// WpaOption is the function signature used to specify options of WpaConfig.
type WpaOption func(*WpaConfig)

// Psk reutrn an WpaOption which sets psk in WPA config.
func Psk(psk string) WpaOption {
	return func(c *WpaConfig) {
		c.Psk = psk
	}
}

// WpaMode reutrn an WpaOption which sets WPA mode in WPA config.
func WpaMode(mode WpaModeEnum) WpaOption {
	return func(c *WpaConfig) {
		c.WpaMode = mode
	}
}

// WpaCiphers return an WpaOption which sets the used WPA cipher in WPA config.
func WpaCiphers(ciphers ...Cipher) WpaOption {
	return func(c *WpaConfig) {
		c.WpaCiphers = append(c.WpaCiphers, ciphers...)
	}
}

// Wpa2Ciphers return an WpaOption which sets the used WPA2 cipher in WPA config.
func Wpa2Ciphers(ciphers ...Cipher) WpaOption {
	return func(c *WpaConfig) {
		c.Wpa2Ciphers = append(c.Wpa2Ciphers, ciphers...)
	}
}

// WpaPtkRekeyPeriod return an WpaOption which sets maximum lifetime for PTK in WPA config.
func WpaPtkRekeyPeriod(period int) WpaOption {
	return func(c *WpaConfig) {
		c.WpaPtkRekeyPeriod = period
	}
}

// WpaGtkRekeyPeriod return an WpaOption which sets time interval for rekeying GTK in WPA config.
func WpaGtkRekeyPeriod(period int) WpaOption {
	return func(c *WpaConfig) {
		c.WpaGtkRekeyPeriod = period
	}
}

// WpaGmkRekeyPeriod return an WpaOption which sets time interval for rekeying GMK in WPA config.
func WpaGmkRekeyPeriod(period int) WpaOption {
	return func(c *WpaConfig) {
		c.WpaGmkRekeyPeriod = period
	}
}

// UseStrictRekey return an WpaOption which sets strict rekey in WPA config.
func UseStrictRekey(use bool) WpaOption {
	return func(c *WpaConfig) {
		c.UseStrictRekey = use
	}
}

// NewWpaConfig creates a WpaConfig with given WPA options.
func NewWpaConfig(ops ...WpaOption) (*WpaConfig, error) {
	// Default config.
	conf := &WpaConfig{
		WpaMode: WpaMixed,
		FTMode:  FTModeNone,
	}
	for _, op := range ops {
		op(conf)
	}
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

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

var _ Config = (*WpaConfig)(nil)

// WpaGenerator holds some WpaOption and provide Gen method to build a new WpaConfig.
type WpaGenerator []WpaOption

// Gen simply call NewWpaConfig but return as interface Config.
func (g WpaGenerator) Gen() (Config, error) {
	return NewWpaConfig(g...)
}

var _ Generator = (WpaGenerator)(nil)

// GetClass returns security class of WPA network.
func (c *WpaConfig) GetClass() string {
	return "psk"
}

// GetHostapdConfig returns hostapd config of WPA network.
func (c *WpaConfig) GetHostapdConfig() (map[string]string, error) {
	ret := map[string]string{"wpa": strconv.Itoa(int(c.WpaMode))}

	switch c.FTMode {
	case FTModeNone:
		ret["wpa_key_mgmt"] = "WPA-PSK"
	case FTModePure:
		ret["wpa_key_mgmt"] = "FT-PSK"
	case FTModeMixed:
		ret["wpa_key_mgmt"] = "WPA-PSK FT-PSK"
	default:
		return nil, errors.Errorf("invalid FTMode %d", int(c.FTMode))
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
		ret["wpa_ptk_rekey"] = strconv.Itoa(c.WpaPtkRekeyPeriod)
	}
	if c.WpaGtkRekeyPeriod != 0 {
		ret["wpa_group_rekey"] = strconv.Itoa(c.WpaGtkRekeyPeriod)
	}
	if c.WpaGmkRekeyPeriod != 0 {
		ret["wpa_gmk_rekey"] = strconv.Itoa(c.WpaGmkRekeyPeriod)
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

// validate validates the WpaConfig.
func (c *WpaConfig) validate() error {
	if c.WpaMode&WpaMixed == 0 {
		return errors.New("cannot configure WPA unless we know which mode to use")
	}
	if c.WpaMode&WpaPure > 0 && len(c.WpaCiphers) == 0 {
		return errors.New("cannot configure WPA unless we have some ciphers")
	}
	if c.WpaMode&Wpa2Pure > 0 && len(c.WpaCiphers) == 0 && len(c.Wpa2Ciphers) == 0 {
		return errors.New("cannot configure WPA2 unless we have some ciphers")
	}
	if len(c.Psk) == 0 {
		return errors.New("cannot configure WPA unless psk is given")
	}
	if len(c.Psk) > 64 {
		return errors.New("wpa passphrases cannot be longer than 63 characters (or 64 hex digits)")
	}
	if len(c.Psk) == 64 {
		isValidHexChar := func(ch rune) bool {
			for _, h := range "0123456789abcdefABCDEF" {
				if ch == h {
					return true
				}
			}
			return false
		}
		for _, ch := range c.Psk {
			if !isValidHexChar(ch) {
				return errors.Errorf("invalid PMK: %q", c.Psk)
			}
		}
	}
	return nil
}

// joinCiphersToString is the utility that concat chiphers with " ".
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
