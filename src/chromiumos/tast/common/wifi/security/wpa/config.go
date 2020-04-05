// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpa provides a Config type for WPA protected network.
package wpa

import (
	"strconv"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// ModeEnum is the type for specifying WPA modes.
type ModeEnum int

// WPA modes.
const (
	ModePure  ModeEnum = 1
	ModePure2 ModeEnum = 2
	ModeMixed          = ModePure | ModePure2
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
type Option func(*Factory)

// Mode returns an Option which sets WPA mode in Config.
func Mode(mode ModeEnum) Option {
	return func(c *Factory) {
		c.Mode = mode
	}
}

// Ciphers returns an Option which sets the used WPA cipher in Config.
func Ciphers(ciphers ...Cipher) Option {
	return func(c *Factory) {
		c.Ciphers = append(c.Ciphers, ciphers...)
	}
}

// Ciphers2 returns an Option which sets the used WPA2 cipher in Config.
func Ciphers2(ciphers ...Cipher) Option {
	return func(c *Factory) {
		c.Ciphers2 = append(c.Ciphers2, ciphers...)
	}
}

// PtkRekeyPeriod returns an Option which sets maximum lifetime for PTK in Config.
func PtkRekeyPeriod(period int) Option {
	return func(c *Factory) {
		c.PtkRekeyPeriod = period
	}
}

// GtkRekeyPeriod returns an Option which sets time interval for rekeying GTK in Config.
func GtkRekeyPeriod(period int) Option {
	return func(c *Factory) {
		c.GtkRekeyPeriod = period
	}
}

// GmkRekeyPeriod returns an Option which sets time interval for rekeying GMK in Config.
func GmkRekeyPeriod(period int) Option {
	return func(c *Factory) {
		c.GmkRekeyPeriod = period
	}
}

// UseStrictRekey returns an Option which sets strict rekey in Config.
func UseStrictRekey(use bool) Option {
	return func(c *Factory) {
		c.UseStrictRekey = use
	}
}

// FTMode returns an Option which sets fast transition mode in Config.
func FTMode(ft FTModeEnum) Option {
	return func(c *Factory) {
		c.FTMode = ft
	}
}

// Config implements security.Config interface for WPA protected network.
type Config struct {
	// Embed base.Config so we don't have to re-implement noop methods.
	base.Config
	Psk            string
	Mode           ModeEnum
	Ciphers        []Cipher
	Ciphers2       []Cipher
	PtkRekeyPeriod int
	GtkRekeyPeriod int
	GmkRekeyPeriod int
	UseStrictRekey bool
	FTMode         FTModeEnum
}

// Static check: Config implements security.Config interface.
var _ security.Config = (*Config)(nil)

// Factory holds some Option and provide Gen method to build a new Config.
type Factory struct {
	Psk            string
	Mode           ModeEnum
	Ciphers        []Cipher
	Ciphers2       []Cipher
	PtkRekeyPeriod int
	GtkRekeyPeriod int
	GmkRekeyPeriod int
	UseStrictRekey bool
	FTMode         FTModeEnum
}

// Gen builds a Config with the given Option stored in Factory.
func (f *Factory) Gen() (security.Config, error) {
	conf := &Config{
		Psk:            f.Psk,
		Mode:           f.Mode,
		PtkRekeyPeriod: f.PtkRekeyPeriod,
		GtkRekeyPeriod: f.GtkRekeyPeriod,
		GmkRekeyPeriod: f.GmkRekeyPeriod,
		UseStrictRekey: f.UseStrictRekey,
		FTMode:         f.FTMode,
	}
	if len(f.Ciphers) != 0 {
		conf.Ciphers = make([]Cipher, len(f.Ciphers))
		copy(conf.Ciphers, f.Ciphers)
	}
	if len(f.Ciphers2) != 0 {
		conf.Ciphers2 = make([]Cipher, len(f.Ciphers2))
		copy(conf.Ciphers2, f.Ciphers2)
	}
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// NewFactory builds a Factory with the given Option.
func NewFactory(psk string, ops ...Option) security.Factory {
	// Default config.
	fac := &Factory{
		Psk:    psk,
		Mode:   ModeMixed,
		FTMode: FTModeNone,
	}
	for _, op := range ops {
		op(fac)
	}
	return fac
}

var _ security.Factory = (*Factory)(nil)

// Class returns security class of WPA network.
func (c *Config) Class() string {
	return "psk"
}

// HostapdConfig returns hostapd config of WPA network.
func (c *Config) HostapdConfig() (map[string]string, error) {
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

	if len(c.Ciphers) != 0 {
		ret["wpa_pairwise"] = joinCiphersToString(c.Ciphers)
	}
	if len(c.Ciphers2) != 0 {
		ret["rsn_pairwise"] = joinCiphersToString(c.Ciphers2)
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

// ShillServiceProperties returns shill properties of WPA network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret := map[string]interface{}{
		shill.ServicePropertyPassphrase: c.Psk,
	}
	if c.FTMode&FTModePure > 0 {
		ret[shill.ServicePropertyFtEnabled] = true
	}
	return ret, nil
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.Mode&ModeMixed == 0 {
		return errors.New("cannot configure WPA unless we know which mode to use")
	}
	if c.Mode&ModePure > 0 && len(c.Ciphers) == 0 {
		return errors.New("cannot configure WPA unless we have some ciphers")
	}
	if c.Mode&ModePure2 > 0 && len(c.Ciphers) == 0 && len(c.Ciphers2) == 0 {
		return errors.New("cannot configure WPA2 unless we have some ciphers")
	}
	if c.Mode&ModePure > 0 && c.Mode&ModePure2 == 0 && len(c.Ciphers2) > 0 {
		return errors.New("Ciphers2 is not supported by pure WPA")
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
