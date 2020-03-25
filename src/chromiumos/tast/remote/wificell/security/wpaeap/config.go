// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpaeap provides implementation of the security common interface of
// WPA tunnel via EAP-TLS negotiation.
package wpaeap

import (
	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/eap"
	"chromiumos/tast/remote/wificell/security/wpa"
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

// UseSystemCAs returns an Option which sets if we should use system cas and ignore server certificates.
func UseSystemCAs(b bool) Option { return inheritOption(eap.UseSystemCAs(b)) }

// ServerCACert returns an Option which sets the PEM encoded CA certificate for server.
func ServerCACert(cert string) Option { return inheritOption(eap.ServerCACert(cert)) }

// ServerCert returns an Option which sets the PEM encoded CA certificate for server.
func ServerCert(cert string) Option { return inheritOption(eap.ServerCert(cert)) }

// ServerCACert returns an Option which sets the PEM encoded CA certificate for server.
func ServerKey(k string) Option { return inheritOption(eap.ServerKey(k)) }

// ServerEAPUsers returns an Option which sets the contents of EAP user file.
func ServerEAPUsers(s string) Option { return inheritOption(eap.ServerEAPUsers(s)) }

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

// EAPIdentity returns an Option which sets the user to authenticate as during EAP.
func EAPIdentity(id string) Option { return inheritOption(eap.EAPIdentity(id)) }

// WPAMode returns an Option which sets WPA mode to use.
func WPAMode(mode wpa.ModeEnum) Option {
	return func(c *Config) {
		c.WPAMode = mode
	}
}

// Config is the WPAEAP implementation of security.Config.
type Config struct {
	*eap.Config
	WPAMode wpa.ModeEnum

	// Temporary buffer for storing options to pass to parent constructor.
	parentOps []eap.Option
}

var _ security.Config = (*Config)(nil)

func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		WPAMode: wpa.ModePure,
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

// GetHostapdConfig returns hostapd config derived from c.
func (c *Config) GetHostapdConfig() (map[string]string, error) {
	ret, err := c.Config.GetHostapdConfig()
	if err != nil {
		return nil, err
	}
	ret["wpa"] = string(c.WPAMode)
	ret["wpa_pairwise"] = string(wpa.CipherCCMP)
	ret["wpa_key_mgmt"] = "WPA-EAP"
	if c.FTMode == wpa.FTModePure {
		ret["wpa_key_mgmt"] = "FT-EAP"
	} else if c.FTMode == wpa.FTModeMixed {
		ret["wpa_key_mgmt"] = "WPA-EAP FT-EAP"
	}
	return ret, nil
}

// Generator holds the Option and provide Gen method to build a new Config.
type Generator []Option

// Gen generates a dynamic WPA EAP Config.
func (g Generator) Gen() (security.Config, error) {
	return NewConfig(g...)
}

var _ security.Generator = (Generator)(nil)
