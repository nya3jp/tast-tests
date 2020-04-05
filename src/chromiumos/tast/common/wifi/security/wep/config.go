// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wep provides a Config type for WEP protected network.
package wep

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// AuthAlgsEnum is the type for specifying WEP authentication algorithms.
type AuthAlgsEnum int

// WEP authentication algorithms modes.
const (
	AuthAlgsOpen AuthAlgsEnum = 1 << iota
	AuthAlgsShared
)

// Option is the function signature used to specify options of Config.
type Option func(*ConfigFactory)

// DefaultKey returns an Option which sets default key in Config.
func DefaultKey(d int) Option {
	return func(f *ConfigFactory) {
		f.blueprint.DefaultKey = d
	}
}

// AuthAlgs returns an Option which sets what authentication algorithm to use in Config.
func AuthAlgs(a AuthAlgsEnum) Option {
	return func(f *ConfigFactory) {
		f.blueprint.AuthAlgs = a
	}
}

// Config implements security.Config interface for WEP protected network.
type Config struct {
	// Embed base.Config so we don't have to re-implement noop methods.
	base.Config
	Keys       []string
	DefaultKey int
	AuthAlgs   AuthAlgsEnum
}

// Static check: Config implements security.Config interface.
var _ security.Config = (*Config)(nil)

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	keys      []string
	blueprint Config
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	conf := f.blueprint
	conf.Keys = make([]string, len(f.keys))

	// To prevent sharing the same array between the different Configs that are
	// Gen-ed by the same ConfigFactory, copy the array in each calling of Gen.
	copy(conf.Keys, f.keys)

	if err := conf.validate(); err != nil {
		return nil, err
	}
	return &conf, nil
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(keys []string, ops ...Option) *ConfigFactory {
	// Default config.
	fac := &ConfigFactory{
		keys:      make([]string, len(keys)),
		blueprint: Config{AuthAlgs: AuthAlgsOpen},
	}
	copy(fac.keys, keys)
	for _, op := range ops {
		op(fac)
	}
	return fac
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)

// Class returns security class of WEP network.
func (c *Config) Class() string {
	return shill.SecurityWEP
}

// HostapdConfig returns hostapd config of WEP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
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

// ShillServiceProperties returns shill properties of WEP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	keyWithIndex := fmt.Sprintf("%d:%s", c.DefaultKey, c.Keys[c.DefaultKey])
	return map[string]interface{}{shill.ServicePropertyPassphrase: keyWithIndex}, nil
}

// formatKey is a helper function for generating hostapd and wpa_cli config.
// formatter is the the function to escape a WEP string-encoded passphrase
// whose format varies depending on the consumer.
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
	for _, key := range c.Keys {
		switch len(key) {
		case 5, 13, 16:
			// No need to check.
		case 10, 26, 32:
			// Just to validate it is a valid hex string, don't need the result.
			if _, err := hex.DecodeString(key); err != nil {
				return errors.Errorf("key with length 10, 26, or 32 should only contain hexadecimal digits: %q", key)
			}
		default:
			return errors.Errorf("invalid key length: %q", key)
		}
	}
	return nil
}
