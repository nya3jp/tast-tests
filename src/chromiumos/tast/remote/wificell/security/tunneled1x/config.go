// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tunneled1x provides implementation of the security common interface of
// TTLS/PEAP.
package tunneled1x

import (
	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/wpaeap"
	"fmt"
	"strings"
)

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// inheritOption wraps a parent option as Option.
func inheritOption(op wpaeap.Option) Option {
	return func(c *Config) {
		c.parentOps = append(c.parentOps, op)
	}
}

// FileSuffix returns an Option which sets file suffix on DUT.
func FileSuffix(suffix string) Option { return inheritOption(wpaeap.FileSuffix(suffix)) }

// ClientPassword returns an Option which sets client password.
func ClientPassword(p string) Option {
	return func(c *Config) {
		c.Password = p
	}
}

// L1Type is the type of Layer 1 (outer) protocol.
type L1Type string

// L1Type enums.
const (
	L1TypePEAP L1Type = "PEAP"
	L1TypeTTLS L1Type = "TTLS"
)

// OuterProtocol returns an Option which sets outer protocol.
func OuterProtocol(p L1Type) Option {
	return func(c *Config) {
		c.Outer = p
	}
}

// L2Type is the type of Layer 2 (inner) protocol.
type L2Type string

// L2Type enums.
const (
	L2TypeGTC           = "GTC"
	L2TypeMSCHAPV2      = "MSCHAPV2"
	L2TypeMD5           = "MD5"
	L2TypeTTLS_MSCHAPV2 = "TTLS-MSCHAPV2"
	L2TypeTTLS_MSCHAP   = "TTLS-MSCHAP"
	L2TypeTTLS_PAP      = "TTLS-PAP"
)

// InnerProtocol returns an Option which sets inner protocol.
func InnerProtocol(p L2Type) Option {
	return func(c *Config) {
		c.Inner = p
	}
}

// Config is the WPAEAP implementation of security.Config.
type Config struct {
	*wpaeap.Config
	Password string
	Outer    L1Type
	Inner    L2Type

	// Temporary buffer for storing options to pass to parent constructor.
	parentOps []wpaeap.Option
}

var _ security.Config = (*Config)(nil)

func NewConfig(serverCA, serverCert, serverKey, clientCA, eapId, passwd string, ops ...Option) (*Config, error) {
	// Default config.
	conf := &Config{
		Password: passwd,
		Outer:    L1TypePEAP,
		Inner:    L2TypeMD5,
	}
	for _, op := range ops {
		op(conf)
	}

	// Generate eap users file content.
	// TODO: verify this is valid.
	eapUsers := strings.Join(
		[]string{
			fmt.Sprintf("* %s", string(conf.Outer)),
			fmt.Sprintf("\"%s\" %s", eapId, string(conf.Inner)),
			fmt.Sprintf("\"%s\" [2]", passwd),
		}, "\n")

	conf.parentOps = append(conf.parentOps,
		wpaeap.ServerCACert(serverCA),
		wpaeap.ServerCert(serverCert),
		wpaeap.ServerKey(serverKey),
		wpaeap.ServerEAPUsers(eapUsers),
		wpaeap.ClientCACert(clientCA),
		wpaeap.EAPIdentity(eapId),
	)

	parent, err := wpaeap.NewConfig(conf.parentOps...)
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

// GetShillServiceProperties returns shill properties of EAP network.
func (c *Config) GetShillServiceProperties() map[string]interface{} {
	ret := c.Config.GetShillServiceProperties()
	ret["EAP.Password"] = c.Password
	// TODO: handle these better.
	if strings.HasPrefix(string(c.Inner), "TTLS-") {
		ret["EAP.InnerEAP"] = fmt.Sprint("auth=", string(c.Inner)[5:])
	}
	return ret
}

// Generator holds the Option and provide Gen method to build a new Config.
// TODO: this might need New function with long required parameters to force
// caller properly assign all fields.
type Generator struct {
	ServerCACert string
	ServerCert   string
	ServerKey    string
	ClientCACert string
	EAPIdentity  string
	Password     string
	Options      []Option
}

// Gen generates a dynamic WPA EAP Config.
func (g *Generator) Gen() (security.Config, error) {
	return NewConfig(
		g.ServerCACert,
		g.ServerCert,
		g.ServerKey,
		g.ClientCACert,
		g.EAPIdentity,
		g.Password,
		g.Options...)
}

var _ security.Generator = (*Generator)(nil)
