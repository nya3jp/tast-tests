// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wep provides a Config type for WEP protected network.
package wep

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
)

// AuthAlgo is the type for specifying WEP authentication algorithms.
type AuthAlgo int

// WEP authentication algorithm modes.
const (
	AuthAlgoOpen AuthAlgo = 1 << iota
	AuthAlgoShared
)

// Config implements security.Config interface for WEP protected network.
type Config struct {
	// Embedded base config so we don't have to re-implement credential-related methods.
	base.Config

	keys       []string
	defaultKey int
	authAlgs   AuthAlgo
}

// Class returns security class of WEP network.
func (c *Config) Class() string {
	return shillconst.SecurityClassWEP
}

// HostapdConfig returns hostapd config of WEP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	ret := make(map[string]string)
	quote := func(s string) string { return fmt.Sprintf("%q", s) }
	if err := c.validateKeys(); err != nil {
		return nil, err
	}
	for i, key := range c.keys {
		formatted, err := formatKey(key, quote)
		if err != nil {
			return nil, err
		}
		ret[fmt.Sprintf("wep_key%d", i)] = formatted
	}
	ret["wep_default_key"] = strconv.Itoa(c.defaultKey)
	ret["auth_algs"] = strconv.Itoa(int(c.authAlgs))
	return ret, nil
}

// ShillServiceProperties returns shill properties of WEP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	keyWithIndex := fmt.Sprintf("%d:%s", c.defaultKey, c.keys[c.defaultKey])
	return map[string]interface{}{shillconst.ServicePropertyPassphrase: keyWithIndex}, nil
}

// formatKey is a helper function for generating hostapd and wpa_cli config.
// formatter is the the function to escape a WEP string-encoded passphrase
// whose format varies depending on the consumer.
func formatKey(key string, formatter func(string) string) (string, error) {
	switch len(key) {
	case 5, 13, 16: // These are 'ASCII' strings, or at least N-byte strings of the right size.
		return formatter(key), nil
	case 10, 26, 32: // These are hex encoded byte strings.
		return key, nil
	default:
		return "", errors.Errorf("invalid key length: %q", key)
	}
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.authAlgs & ^(AuthAlgoOpen|AuthAlgoShared) > 0 {
		return errors.New("invalid WEP auth algorithm is set")
	}
	if c.authAlgs&(AuthAlgoOpen|AuthAlgoShared) == 0 {
		return errors.New("no WEP auth algorithm is set")
	}
	if len(c.keys) > 4 {
		return errors.Errorf("at most 4 keys can be set, got %d keys", len(c.keys))
	}
	if c.defaultKey >= len(c.keys) || c.defaultKey < 0 {
		return errors.Errorf("default key index %d out of range %d", c.defaultKey, len(c.keys))
	}
	if err := c.validateKeys(); err != nil {
		return err
	}
	return nil
}

// validateKeys validates the keys.
func (c *Config) validateKeys() error {
	for _, key := range c.keys {
		switch len(key) {
		case 5, 13, 16: // These are 'ASCII' strings, or at least N-byte strings of the right size.
			// No need to check.
		case 10, 26, 32: // These are hex encoded byte strings.
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

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	keys []string
	ops  []Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(keys []string, ops ...Option) *ConfigFactory {
	return &ConfigFactory{
		keys: append([]string(nil), keys...),
		ops:  ops,
	}
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	// Default config.
	conf := &Config{
		keys:     f.keys,
		authAlgs: AuthAlgoOpen,
	}

	for _, op := range f.ops {
		op(conf)
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
