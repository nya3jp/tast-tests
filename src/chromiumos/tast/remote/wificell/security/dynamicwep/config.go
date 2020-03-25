// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dynamicwep provides dynamic WEP implementation of the security common interface.
package dynamicwep

import (
	"strconv"

	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/eap"
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// inheritOption wraps a parent option as Option.
func inheritOption(op eap.Option) Option {
	return func(c *Config) {
		c.parentOps = append(c.parentOps, op)
	}
}

// FileSuffix returns an Option which sets file suffix on DUT.
func FileSuffix(suffix string) Option { return inheritOption(eap.FileSuffix(suffix)) }

// ServerCACert returns an Option which sets the PEM encoded CA certificate for server.
func ServerCACert(cert string) Option { return inheritOption(eap.ServerCACert(cert)) }

// ServerCert returns an Option which sets the PEM encoded CA certificate for server.
func ServerCert(cert string) Option { return inheritOption(eap.ServerCert(cert)) }

// ServerCACert returns an Option which sets the PEM encoded CA certificate for server.
func ServerKey(k string) Option { return inheritOption(eap.ServerKey(k)) }

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client.
func ClientCACert(cert string) Option { return inheritOption(eap.ClientCACert(cert)) }

// ClientCert returns an Option which sets the PEM encoded CA certificate for client.
func ClientCert(cert string) Option { return inheritOption(eap.ClientCert(cert)) }

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client.
func ClientKey(k string) Option { return inheritOption(eap.ClientKey(k)) }

// ClientCertID returns an Option which sets the identifier for client certificate in TPM.
func ClientCertID(id string) Option { return inheritOption(eap.ClientCertID(id)) }

// ClientKeyID returns an Option which sets the identifier for client private key in TPM.
func ClientKeyID(id string) Option { return inheritOption(eap.ClientKeyID(id)) }

// UseShortKeys returns an Option which sets if we should force hostapd to use 40 bit WEP keys.
func UseShortKeys(b bool) Option {
	return func(c *Config) {
		c.UseShortKeys = b
	}
}

// RekeyPeriod returns an Option which sets the duration in seconds  between rekeys.
func RekeyPeriod(sec int) Option {
	return func(c *Config) {
		c.RekeyPeriod = sec
	}
}

// Config is the dynamic WEP implementation of security.Config.
type Config struct {
	*eap.Config
	UseShortKeys bool
	RekeyPeriod  int

	// Temporary buffer for storing options to pass to parent constructor.
	parentOps []eap.Option
}

var _ security.Config = (*Config)(nil)

func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		RekeyPeriod: 20,
	}
	for _, op := range ops {
		op(conf)
	}
	parent, err := eap.NewConfig(conf.parentOps...)
	if err != nil {
		return nil, err
	}
	conf.Config = parent
	conf.parentOps = nil

	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

func (c *Config) validate() error {
	// TODO:
	return nil
}

// GetClass returns security class of dynamic WEP.
func (c *Config) GetClass() string {
	return "wep"
}

// GetHostapdConfig returns hostapd config derived from c.
func (c *Config) GetHostapdConfig() (map[string]string, error) {
	ret, err := c.Config.GetHostapdConfig()
	if err != nil {
		return nil, err
	}
	if c.UseShortKeys {
		ret["wep_key_len_broadcast"] = "5"
		ret["wep_key_len_unicast"] = "5"
	} else {
		ret["wep_key_len_broadcast"] = "13"
		ret["wep_key_len_unicast"] = "13"
	}
	ret["wep_rekey_period"] = strconv.Itoa(c.RekeyPeriod)
	return ret, nil
}

// GetShillServiceProperties returns shill properties derived from c.
func (c *Config) GetShillServiceProperties() map[string]interface{} {
	ret := c.Config.GetShillServiceProperties()
	ret["EAP.KeyMgmt"] = "IEEE8021X"
	return ret
}

// Generator holds the Option and provide Gen method to build a new Config.
type Generator []Option

// Gen generates a dynamic WEP Config.
func (g Generator) Gen() (security.Config, error) {
	return NewConfig(g...)
}

var _ security.Generator = (Generator)(nil)
