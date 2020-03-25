// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wep provides WEP implementation of the security common interface.
package wep

import (
	"fmt"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/remote/wificell/security"
)

// AuthAlgsEnum is the type for specifying WEP authentication algorithms.
type AuthAlgsEnum int

// WEP authentication algorithms modes.
const (
	AuthAlgsOpen   = 1
	AuthAlgsShared = 2
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// Keys returns a Option which sets keys in WEP config.
func Keys(strs []string) Option {
	return func(c *Config) {
		c.Keys = make([]string, len(strs))
		copy(c.Keys, strs)
	}
}

// DefaultKey returns a Option which sets default key in WEP config.
func DefaultKey(d int) Option {
	return func(c *Config) {
		c.DefaultKey = d
	}
}

// AuthAlgs returns a Option which sets what authentication algorithm to use in WEP config.
func AuthAlgs(a AuthAlgsEnum) Option {
	return func(c *Config) {
		c.AuthAlgs = a
	}
}

// NewConfig creates a Config with given WEP options.
func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{AuthAlgs: AuthAlgsOpen}
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
	Keys       []string
	DefaultKey int
	AuthAlgs   AuthAlgsEnum
}

var _ security.Config = (*Config)(nil)

// Generator holds some Option and provide Gen method to build a new Config.
type Generator []Option

// Gen simply calls NewConfig but returns as interface Config.
func (g Generator) Gen() (security.Config, error) {
	return NewConfig(g...)
}

var _ security.Generator = (Generator)(nil)

// GetClass returns security class of WEP network.
func (c *Config) GetClass() string {
	return "wep"
}

// GetHostapdConfig returns hostapd config of WEP network.
func (c *Config) GetHostapdConfig() (map[string]string, error) {
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
func (c *Config) GetShillServiceProperties() map[string]interface{} {
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

// validate validates the Config.
func (c *Config) validate() error {
	if c.AuthAlgs & ^(AuthAlgsOpen|AuthAlgsShared) > 0 {
		return errors.New("invalid WEP auth algorithm is set")
	}
	if c.AuthAlgs&(AuthAlgsOpen|AuthAlgsShared) == 0 {
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
func (c *Config) validateKeys() error {
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
