// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpa provides a Config type for WPA protected network.
package wpa

import (
	"encoding/hex"
	"strconv"
	"strings"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
)

// A PSK should be a string composed with 64 hex digits, or a ASCII passphrase whose length is between 8 and 63 (inclusive).
const (
	RawPSKLen        = 64
	MinPassphraseLen = 8
	MaxPassphraseLen = 63
)

// ModeEnum is the type for specifying WPA modes.
type ModeEnum int

// WPA modes.
const (
	ModePureWPA ModeEnum = 1 << iota
	ModePureWPA2
	ModePureWPA3
	ModeMixed     = ModePureWPA | ModePureWPA2
	ModeMixedWPA3 = ModePureWPA2 | ModePureWPA3
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
	// Embedded base config so we don't have to re-implement credential-related methods.
	base.Config

	psk            string
	mode           ModeEnum
	ciphers        []Cipher // ciphers used for WPA.
	ciphers2       []Cipher // ciphers used for WPA2.
	ptkRekeyPeriod int
	gtkRekeyPeriod int
	gmkRekeyPeriod int
	useStrictRekey bool
	ftMode         FTModeEnum
}

// Class returns security class of WPA network.
func (c *Config) Class() string {
	return shillconst.SecurityPSK
}

// PSK returns the passphrase for WPA network.
func (c *Config) PSK() string {
	return c.psk
}

// Ciphers2 returns WPA2 ciphers of the network.
func (c *Config) Ciphers2() string {
	var ciphersstr []string
	for _, cipher := range c.ciphers2 {
		ciphersstr = append(ciphersstr, string(cipher))
	}

	return string(strings.Join(ciphersstr, " "))
}

// HostapdConfig returns hostapd config of WPA network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	var ret = make(map[string]string)

	var mode int
	// WPA2 and WPA3 are both RSN, and share the same bit in wpa=.
	if c.mode&(ModePureWPA2|ModePureWPA3) > 0 {
		mode |= int(ModePureWPA2)
	}
	if c.mode&ModePureWPA > 0 {
		mode |= int(ModePureWPA)
	}
	ret["wpa"] = strconv.Itoa(mode)

	var keyMgmt []string
	if c.ftMode&FTModeNone > 0 {
		if c.mode&ModeMixed > 0 {
			keyMgmt = append(keyMgmt, "WPA-PSK")
		}
		if c.mode&ModePureWPA3 > 0 {
			keyMgmt = append(keyMgmt, "SAE")
		}
	}
	if c.ftMode&FTModePure > 0 {
		if c.mode&ModeMixed > 0 {
			keyMgmt = append(keyMgmt, "FT-PSK")
		}
		if c.mode&ModePureWPA3 > 0 {
			keyMgmt = append(keyMgmt, "FT-SAE")
		}
	}
	ret["wpa_key_mgmt"] = strings.Join(keyMgmt, " ")

	if len(c.psk) == RawPSKLen {
		ret["wpa_psk"] = c.psk
	} else {
		ret["wpa_passphrase"] = c.psk
	}

	if len(c.ciphers) != 0 {
		ret["wpa_pairwise"] = concatCiphers(c.ciphers)
	}
	if len(c.ciphers2) != 0 {
		ret["rsn_pairwise"] = concatCiphers(c.ciphers2)
	}

	if c.ptkRekeyPeriod != 0 {
		ret["wpa_ptk_rekey"] = strconv.Itoa(c.ptkRekeyPeriod)
	}
	if c.gtkRekeyPeriod != 0 {
		ret["wpa_group_rekey"] = strconv.Itoa(c.gtkRekeyPeriod)
	}
	if c.gmkRekeyPeriod != 0 {
		ret["wpa_gmk_rekey"] = strconv.Itoa(c.gmkRekeyPeriod)
	}

	if c.useStrictRekey {
		ret["wpa_strict_rekey"] = "1"
	}

	return ret, nil
}

// ShillServiceProperties returns shill properties of WPA network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret := map[string]interface{}{
		shillconst.ServicePropertyPassphrase: c.psk,
	}
	return ret, nil
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.mode&(^(ModeMixed|ModeMixedWPA3)) > 0 || c.mode == 0 {
		return errors.Errorf("invalid mode %d", c.mode)
	}
	if c.mode&ModePureWPA > 0 && len(c.ciphers) == 0 {
		return errors.New("cannot configure WPA unless we have some ciphers")
	}
	if c.mode&ModeMixedWPA3 > 0 && len(c.ciphers) == 0 && len(c.ciphers2) == 0 {
		return errors.New("cannot configure RSN (WPA2/WPA3) unless we have some ciphers")
	}
	if c.mode&ModePureWPA > 0 && c.mode&ModePureWPA2 == 0 && len(c.ciphers2) > 0 {
		return errors.New("ciphers2 is not supported by pure WPA")
	}
	if c.ftMode&(^FTModeMixed) > 0 || c.ftMode == 0 {
		return errors.Errorf("invalid ftMode %d", c.ftMode)
	}
	if err := c.validatePSK(); err != nil {
		return err
	}
	return nil
}

// validatePSK validates the PSK.
func (c *Config) validatePSK() error {
	if len(c.psk) == 0 {
		return errors.New("cannot configure WPA unless PSK is given")
	}
	if len(c.psk) > RawPSKLen {
		return errors.New("WPA passphrases cannot be longer than 63 characters (or 64 hex digits)")
	}
	if len(c.psk) < MinPassphraseLen {
		return errors.New("WPA passphrases should be longer than 8 characters")
	}
	if len(c.psk) == RawPSKLen {
		// Just to validate it is a valid hex string, don't need the result.
		if _, err := hex.DecodeString(c.psk); err != nil {
			return errors.Errorf("invalid PMK: %q", c.psk)
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

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	psk string
	ops []Option
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	// Default config.
	conf := &Config{
		psk:    f.psk,
		mode:   ModeMixed,
		ftMode: FTModeNone,
	}

	for _, op := range f.ops {
		op(conf)
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(psk string, ops ...Option) *ConfigFactory {
	return &ConfigFactory{
		psk: psk,
		ops: ops,
	}
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
