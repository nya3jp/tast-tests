// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpa provides WPA implementation of the security common interfaces.
package wpa

import (
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/remote/wificell/security"
)

// ModeEnum is the type for specifying WPA modes.
type ModeEnum int

// WPA modes.
const (
	// ModePure is the mode of pure WPA.
	ModePure ModeEnum = 1
	// ModePure is the mode of pure WPA2.
	ModePure2 ModeEnum = 2
	// ModePure is the mixed mode.
	ModeMixed = ModePure | ModePure2
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

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// Psk returns a Option which sets psk in WPA config.
func Psk(psk string) Option {
	return func(c *Config) {
		c.Psk = psk
	}
}

// Mode returns a Option which sets WPA mode in WPA config.
func Mode(mode ModeEnum) Option {
	return func(c *Config) {
		c.Mode = mode
	}
}

// Ciphers returns a Option which sets the used WPA cipher in WPA config.
func Ciphers(ciphers ...Cipher) Option {
	return func(c *Config) {
		c.WpaCiphers = append(c.WpaCiphers, ciphers...)
	}
}

// Ciphers2 returns a Option which sets the used WPA2 cipher in WPA config.
func Ciphers2(ciphers ...Cipher) Option {
	return func(c *Config) {
		c.Wpa2Ciphers = append(c.Wpa2Ciphers, ciphers...)
	}
}

// PtkRekeyPeriod returns a Option which sets maximum lifetime for PTK in WPA config.
func PtkRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.PtkRekeyPeriod = period
	}
}

// GtkRekeyPeriod returns a Option which sets time interval for rekeying GTK in WPA config.
func GtkRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.GtkRekeyPeriod = period
	}
}

// GmkRekeyPeriod returns a Option which sets time interval for rekeying GMK in WPA config.
func GmkRekeyPeriod(period int) Option {
	return func(c *Config) {
		c.GmkRekeyPeriod = period
	}
}

// UseStrictRekey returns a Option which sets strict rekey in WPA config.
func UseStrictRekey(use bool) Option {
	return func(c *Config) {
		c.UseStrictRekey = use
	}
}

// FTMode returns a Option which sets fast transition mode in WPA config.
func FTMode(ft FTModeEnum) Option {
	return func(c *Config) {
		c.FTMode = ft
	}
}

// NewConfig creates a Config with given WPA options.
func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		Mode:   ModeMixed,
		FTMode: FTModeNone,
	}
	for _, op := range ops {
		op(conf)
	}
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// Config is the security config used to start hostapd and set properties of DUT.
type Config struct {
	// Embed BaseConfig so we don't have to re-implement noop methods.
	security.BaseConfig
	Psk            string
	Mode           ModeEnum
	WpaCiphers     []Cipher
	Wpa2Ciphers    []Cipher
	PtkRekeyPeriod int
	GtkRekeyPeriod int
	GmkRekeyPeriod int
	UseStrictRekey bool
	FTMode         FTModeEnum
}

var _ security.Config = (*Config)(nil)

// Generator holds some Option and provide Gen method to build a new Config.
type Generator []Option

// Gen simply calls NewConfig but returns as interface Config.
func (g Generator) Gen() (security.Config, error) {
	return NewConfig(g...)
}

var _ security.Generator = (Generator)(nil)

// GetClass returns security class of WPA network.
func (c *Config) GetClass() string {
	return "psk"
}

// GetHostapdConfig returns hostapd config of WPA network.
func (c *Config) GetHostapdConfig() (map[string]string, error) {
	ret := map[string]string{"wpa": strconv.Itoa(int(c.Mode))}

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

	if err := c.validatePsk(); err != nil {
		return nil, err
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

	if c.PtkRekeyPeriod != 0 {
		ret["wpa_ptk_rekey"] = strconv.Itoa(c.PtkRekeyPeriod)
	}
	if c.GtkRekeyPeriod != 0 {
		ret["wpa_group_rekey"] = strconv.Itoa(c.GtkRekeyPeriod)
	}
	if c.GmkRekeyPeriod != 0 {
		ret["wpa_gmk_rekey"] = strconv.Itoa(c.GmkRekeyPeriod)
	}

	if c.UseStrictRekey {
		ret["wpa_strict_rekey"] = "1"
	}

	return ret, nil
}

// GetShillServiceProperties returns shill properties of WPA network.
func (c *Config) GetShillServiceProperties() map[string]interface{} {
	ret := map[string]interface{}{
		shill.ServicePropertyPassphrase: c.Psk,
	}
	if c.FTMode&FTModePure > 0 {
		ret[shill.ServicePropertyFtEnabled] = true
	}
	return ret
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.Mode&ModeMixed == 0 {
		return errors.New("cannot configure WPA unless we know which mode to use")
	}
	if c.Mode&ModePure > 0 && len(c.WpaCiphers) == 0 {
		return errors.New("cannot configure WPA unless we have some ciphers")
	}
	if c.Mode&ModePure2 > 0 && len(c.WpaCiphers) == 0 && len(c.Wpa2Ciphers) == 0 {
		return errors.New("cannot configure WPA2 unless we have some ciphers")
	}
	if c.Mode&ModePure > 0 && c.Mode&ModePure2 == 0 && len(c.Wpa2Ciphers) > 0 {
		return errors.New("Wpa2Ciphers is not supported by pure WPA")
	}
	if err := c.validatePsk(); err != nil {
		return err
	}
	return nil
}

// validatePsk validates the PSK.
func (c *Config) validatePsk() error {
	if len(c.Psk) == 0 {
		return errors.New("cannot configure WPA unless psk is given")
	}
	if len(c.Psk) > 64 {
		return errors.New("WPA passphrases cannot be longer than 63 characters (or 64 hex digits)")
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
