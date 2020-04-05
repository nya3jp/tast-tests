// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpa provides a Config type for WPA protected network.
package wpa

import (
	"encoding/hex"
	"strconv"
	"strings"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// A PSK should be a string composed with 64 hex digits, or a ASCII passphrase whose length is between 8 and 63 (inclusive).
const (
	PSKLen           = 64
	MinPassphraseLen = 8
	MaxPassphraseLen = 63
)

// ModeEnum is the type for specifying WPA modes.
type ModeEnum int

// WPA modes.
const (
	ModePureWPA ModeEnum = 1 << iota
	ModePureWPA2
	ModeMixed = ModePureWPA | ModePureWPA2
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
	FTModeNone FTModeEnum = 1 << iota
	FTModePure
	FTModeMixed = FTModeNone | FTModePure
)

// Config implements security.Config interface for WPA protected network.
type Config struct {
	// Embed base.Config so we don't have to re-implement noop methods.
	base.Config
	PSK            string
	Mode           ModeEnum
	CiphersWPA     []Cipher
	CiphersWPA2    []Cipher
	PTKRekeyPeriod int
	GTKRekeyPeriod int
	GMKRekeyPeriod int
	UseStrictRekey bool
	FTMode         FTModeEnum
}

// Static check: Config implements security.Config interface.
var _ security.Config = (*Config)(nil)

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	ciphers   []Cipher
	ciphers2  []Cipher
	blueprint Config
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	conf := f.blueprint

	if len(f.ciphers) != 0 {
		conf.CiphersWPA = make([]Cipher, len(f.ciphers))
		copy(conf.CiphersWPA, f.ciphers)
	}
	if len(f.ciphers2) != 0 {
		conf.CiphersWPA2 = make([]Cipher, len(f.ciphers2))
		copy(conf.CiphersWPA2, f.ciphers2)
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}
	return &conf, nil
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(psk string, ops ...Option) *ConfigFactory {
	// Default config.
	fac := &ConfigFactory{
		blueprint: Config{
			PSK:    psk,
			Mode:   ModeMixed,
			FTMode: FTModeNone,
		},
	}
	for _, op := range ops {
		op(fac)
	}
	return fac
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)

// Class returns security class of WPA network.
func (c *Config) Class() string {
	return shill.SecurityPSK
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

	if err := c.validatePSK(); err != nil {
		return nil, err
	}
	if len(c.PSK) == PSKLen {
		ret["wpa_psk"] = c.PSK
	} else {
		ret["wpa_passphrase"] = c.PSK
	}

	if len(c.CiphersWPA) != 0 {
		ret["wpa_pairwise"] = concatCiphers(c.CiphersWPA)
	}
	if len(c.CiphersWPA2) != 0 {
		ret["rsn_pairwise"] = concatCiphers(c.CiphersWPA2)
	}

	if c.PTKRekeyPeriod != 0 {
		ret["wpa_ptk_rekey"] = strconv.Itoa(c.PTKRekeyPeriod)
	}
	if c.GTKRekeyPeriod != 0 {
		ret["wpa_group_rekey"] = strconv.Itoa(c.GTKRekeyPeriod)
	}
	if c.GMKRekeyPeriod != 0 {
		ret["wpa_gmk_rekey"] = strconv.Itoa(c.GMKRekeyPeriod)
	}

	if c.UseStrictRekey {
		ret["wpa_strict_rekey"] = "1"
	}

	return ret, nil
}

// ShillServiceProperties returns shill properties of WPA network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret := map[string]interface{}{
		shill.ServicePropertyPassphrase: c.PSK,
	}
	if c.FTMode&FTModePure > 0 {
		ret[shill.ServicePropertyFTEnabled] = true
	}
	return ret, nil
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.Mode&ModeMixed == 0 {
		return errors.New("cannot configure WPA unless we know which mode to use")
	}
	if c.Mode&ModePureWPA > 0 && len(c.CiphersWPA) == 0 {
		return errors.New("cannot configure WPA unless we have some ciphers")
	}
	if c.Mode&ModePureWPA2 > 0 && len(c.CiphersWPA) == 0 && len(c.CiphersWPA2) == 0 {
		return errors.New("cannot configure WPA2 unless we have some ciphers")
	}
	if c.Mode&ModePureWPA > 0 && c.Mode&ModePureWPA2 == 0 && len(c.CiphersWPA2) > 0 {
		return errors.New("CiphersWPA2 is not supported by pure WPA")
	}
	if c.FTMode&(^FTModeMixed) > 0 || c.FTMode == 0 {
		return errors.Errorf("invalid FTMode %d", int(c.FTMode))
	}
	if err := c.validatePSK(); err != nil {
		return err
	}
	return nil
}

// validatePSK validates the PSK.
func (c *Config) validatePSK() error {
	if len(c.PSK) == 0 {
		return errors.New("cannot configure WPA unless PSK is given")
	}
	if len(c.PSK) > PSKLen {
		return errors.New("WPA passphrases cannot be longer than 63 characters (or 64 hex digits)")
	}
	if len(c.PSK) < MinPassphraseLen {
		return errors.New("WPA passphrases should be longer than 8 characters")
	}
	if len(c.PSK) == PSKLen {
		// Just to validate it is a valid hex string, don't need the result.
		if _, err := hex.DecodeString(c.PSK); err != nil {
			return errors.Errorf("invalid PMK: %q", c.PSK)
		}
	}
	return nil
}

// concatCiphers is the utility that concat chiphers with " ".
func concatCiphers(ciphers []Cipher) string {
	var b strings.Builder
	for _, c := range ciphers {
		if b.Len() != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(string(c))
	}
	return b.String()
}
