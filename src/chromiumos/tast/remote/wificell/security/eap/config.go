// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package eap provides EAP implementation of the security common interface.
package eap

import (
	"chromiumos/tast/local/shill"
	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/wpa"
	"fmt"
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// FileSuffix returns an Option which sets file suffix on DUT.
func FileSuffix(suffix string) Option {
	return func(c *Config) {
		c.FileSuffix = suffix
	}
}

// UseSystemCAs returns an Option which sets if we should use system cas and ignore server certificates.
func UseSystemCAs(b bool) Option {
	return func(c *Config) {
		c.UseSystemCAs = b
	}
}

// ServerCACert returns an Option which sets the PEM encoded CA certificate for server.
func ServerCACert(cert string) Option {
	return func(c *Config) {
		c.ServerCACert = cert
	}
}

// ServerCert returns an Options which sets the PEM encoded identity certificate for server.
func ServerCert(cert string) Option {
	return func(c *Config) {
		c.ServerCert = cert
	}
}

// ServerKey returns an Option which sets the PEM encoded private key for server.
func ServerKey(k string) Option {
	return func(c *Config) {
		c.ServerKey = k
	}
}

// ServerEAPUsers returns an Option which sets the contents of EAP user file.
func ServerEAPUsers(s string) Option {
	return func(c *Config) {
		c.ServerEAPUsers = s
	}
}

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client.
func ClientCACert(cert string) Option {
	return func(c *Config) {
		c.ClientCACert = cert
	}
}

// ClientCert returns an Options which sets the PEM encoded identity certificate for client.
func ClientCert(cert string) Option {
	return func(c *Config) {
		c.ClientCert = cert
	}
}

// ClientKey returns an Option which sets the PEM encoded private key for client.
func ClientKey(k string) Option {
	return func(c *Config) {
		c.ClientKey = k
	}
}

// ClientCertID returns an Option which sets the identifier for client certificate in TPM.
func ClientCertID(id string) Option {
	return func(c *Config) {
		c.ClientCertID = id
	}
}

// ClientKeyID returns an Option which sets the identifier for client private key in TPM.
func ClientKeyID(id string) Option {
	return func(c *Config) {
		c.ClientKeyID = id
	}
}

// EAPIdentity returns an Option which sets the user to authenticate as during EAP.
func EAPIdentity(id string) Option {
	return func(c *Config) {
		c.EAPIdentity = id
	}
}

// FTMode returns an Option which sets the ft mode in EAP config.
// TODO: maybe wpa.FTModeEnum should be moved to a common package.
func FTMode(mode wpa.FTModeEnum) Option {
	return func(c *Config) {
		c.FTMode = mode
	}
}

// TODO: altsubject_match

// Config is the EAP implementation of security.Config.
type Config struct {
	security.BaseConfig

	FileSuffix     string
	UseSystemCAs   bool
	ServerCACert   string
	ServerCert     string
	ServerKey      string
	ServerEAPUsers string
	ClientCACert   string
	ClientCert     string
	ClientKey      string
	ClientCertID   string
	ClientKeyID    string
	EAPIdentity    string
	FTMode         wpa.FTModeEnum

	serverCACertFile  string
	serverCertFile    string
	serverKeyFile     string
	serverEAPUserFile string
}

var _ security.Config = (*Config)(nil)

func NewConfig(ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		FileSuffix:     "randsuffix", // TODO: gen random suffix.
		ServerEAPUsers: "* TLS",
		EAPIdentity:    "chromeos",
		FTMode:         wpa.FTModeNone,
	}
	for _, op := range ops {
		op(conf)
	}
	// TODO: post-processing
	for _, it := range []struct {
		prefix string
		target *string
	}{
		// TODO: maybe install into working dir.
		{"/tmp/hostapd_ca_cert_file.", &conf.serverCACertFile},
		{"/tmp/hostapd_cert_file.", &conf.serverCertFile},
		{"/tmp/hostapd_key_file.", &conf.serverKeyFile},
		{"/tmp/hostapd_eap_user_file.", &conf.serverEAPUserFile},
	} {
		*it.target = fmt.Sprint(it.prefix, conf.FileSuffix)
	}
	if err := conf.validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

func (c *Config) validate() error {
	// TODO:
	return nil
}

// GetClass returns security class of EAP.
func (c *Config) GetClass() string {
	return "802_1x"
}

// GetHostapdConfig returns hostapd config of EAP network.
func (c *Config) GetHostapdConfig() (map[string]string, error) {
	return map[string]string{
		"ieee8021x":     "1",
		"eap_server":    "1",
		"ca_cert":       c.serverCACertFile,
		"server_cert":   c.serverCertFile,
		"private_key":   c.serverKeyFile,
		"eap_user_file": c.serverEAPUserFile,
	}, nil
}

// GetShillServiceProperties returns shill properties of EAP network.
func (c *Config) GetShillServiceProperties() map[string]interface{} {
	ret := map[string]interface{}{
		"EAP.Identity": c.EAPIdentity,
	}
	// TODO: tpm pin.
	if c.ClientCACert != "" {
		ret["EAP.CACertPEM"] = c.ClientCACert
	}
	// TODO: client cert
	// TODO: client key
	if c.FTMode&wpa.FTModePure > 0 {
		ret[shill.ServicePropertyFtEnabled] = true
	}
	return ret
}

// Generator holds the Option and provide Gen method to build a new Config.
type Generator []Option

// Gen generates a EAP Config.
func (g Generator) Gen() (security.Config, error) {
	return NewConfig(g...)
}

var _ security.Generator = (Generator)(nil)
