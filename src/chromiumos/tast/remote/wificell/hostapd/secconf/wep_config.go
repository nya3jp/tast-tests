// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"fmt"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// WepAuthAlgsEnum is the type for specifying WEP authentication algorithms.
type WepAuthAlgsEnum int

// WEP authentication algorithms modes.
const (
	WepAuthAlgsOpen   = 1
	WepAuthAlgsShared = 2
)

// WepOption is the function signature used to specify options of WepConfig.
type WepOption func(*WepConfig)

// WepKeys returns a WepOption which sets keys in WEP config.
func WepKeys(strs []string) WepOption {
	return func(c *WepConfig) {
		c.Keys = make([]string, len(strs))
		copy(c.Keys, strs)
	}
}

// WepDefaultKey returns a WepOption which sets default key in WEP config.
func WepDefaultKey(d int) WepOption {
	return func(c *WepConfig) {
		c.DefaultKey = d
	}
}

// WepAuthAlgs returns a WepOption which sets what authentication algorithm to use in WEP config.
func WepAuthAlgs(a WepAuthAlgsEnum) WepOption {
	return func(c *WepConfig) {
		c.AuthAlgs = a
	}
}

// NewWepConfig creates a WepConfig with given WEP options.
func NewWepConfig(ops ...WepOption) (*WepConfig, error) {
	// Default config.
	conf := &WepConfig{AuthAlgs: WepAuthAlgsOpen}
	for _, op := range ops {
		op(conf)
	}
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// WepConfig is the security config used to start hostapd and set properties of DUT.
type WepConfig struct {
	// Embed BaseConfig so we don't have to re-implement noop methods.
	BaseConfig
	Keys       []string
	DefaultKey int
	AuthAlgs   WepAuthAlgsEnum
}

var _ Config = (*WepConfig)(nil)

// WepGenerator holds some WepOption and provide Gen method to build a new WepConfig.
type WepGenerator []WepOption

// Gen simply calls NewWepConfig but returns as interface Config.
func (g WepGenerator) Gen() (Config, error) {
	return NewWepConfig(g...)
}

var _ Generator = (WepGenerator)(nil)

// GetClass returns security class of WEP network.
func (c *WepConfig) GetClass() string {
	return "wep"
}

// GetHostapdConfig returns hostapd config of WEP network.
func (c *WepConfig) GetHostapdConfig() (map[string]string, error) {
	ret := make(map[string]string)
	quote := func(s string) string { return fmt.Sprintf("%q", s) }
	if err := c.validateKeys(); err != nil {
		return nil, err
	}
	for i, key := range c.Keys {
		formatted, err := formatKey(key, quote)
		if err != nil {
			return nil, err
		}
		ret[fmt.Sprintf("wep_key%d", i)] = formatted
	}
	ret["wep_default_key"] = strconv.Itoa(c.DefaultKey)
	ret["auth_algs"] = strconv.Itoa(int(c.AuthAlgs))
	return ret, nil
}

// GetShillServiceProperties returns shill properties of WEP network.
func (c *WepConfig) GetShillServiceProperties() map[string]interface{} {
	keyWithIndex := fmt.Sprintf("%d:%s", c.DefaultKey, c.Keys[c.DefaultKey])
	return map[string]interface{}{shill.ServicePropertyPassphrase: keyWithIndex}
}

// formatKey is a helper function for generating hostapd and wpa_cli config.
func formatKey(key string, formatter func(string) string) (string, error) {
	switch len(key) {
	case 5, 13, 16:
		return formatter(key), nil
	case 10, 26, 32:
		return key, nil
	default:
		return "", errors.Errorf("invalid key length: %q", key)
	}
}

// validate validates the WepConfig.
func (c *WepConfig) validate() error {
	if c.AuthAlgs & ^(WepAuthAlgsOpen|WepAuthAlgsShared) > 0 {
		return errors.New("invalid WEP auth algorithm is set")
	}
	if c.AuthAlgs&(WepAuthAlgsOpen|WepAuthAlgsShared) == 0 {
		return errors.New("no WEP auth algorithm is set")
	}
	if len(c.Keys) > 4 {
		return errors.Errorf("at most 4 keys can be set, got %d keys", len(c.Keys))
	}
	if c.DefaultKey >= len(c.Keys) || c.DefaultKey < 0 {
		return errors.Errorf("default key index %d out of range %d", c.DefaultKey, len(c.Keys))
	}
	if err := c.validateKeys(); err != nil {
		return err
	}
	return nil
}

// validateKeys validates the keys.
func (c *WepConfig) validateKeys() error {
	isValidHexChar := func(ch rune) bool {
		for _, v := range "0123456789abcdefABCDEF" {
			if ch == v {
				return true
			}
		}
		return false
	}
	for _, key := range c.Keys {
		switch len(key) {
		case 5, 13, 16:
			// No need to check.
		case 10, 26, 32:
			for _, ch := range key {
				if !isValidHexChar(ch) {
					return errors.Errorf("key with length 10, 26, or 32 should only contain hexadecimal digits: %q", key)
				}
			}
		default:
			return errors.Errorf("invalid key length: %q", key)
		}
	}
	return nil
}
