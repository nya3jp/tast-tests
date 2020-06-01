// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dynamicwep provides a Config type for Dynamic WEP protected network.
package dynamicwep

import (
	"strconv"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// defaultRekeyPeriod is the default rekey period in seconds of Dynamic WEP.
const defaultRekeyPeriod = 20

// Config implements security.Config interface for Dynamic WEP protected network.
type Config struct {
	// Embedded EAP Config to inherit the Install* methods.
	*eap.Config

	useShortKey bool
	rekeyPeriod int
}

// Class returns security class of DynamicWEP network.
func (c *Config) Class() string {
	return shill.SecurityWEP
}

// HostapdConfig returns hostapd config of DynamicWEP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	ret, err := c.Config.HostapdConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hostapd config from underlying EAP Config")
	}
	keyLen := 13 // 128 bit WEP with 104 secret bits.
	if c.useShortKey {
		keyLen = 5 // 64 bit WEP with 40 secret bits.
	}
	ret["wep_key_len_broadcast"] = strconv.Itoa(keyLen)
	ret["wep_key_len_unicast"] = strconv.Itoa(keyLen)
	ret["wep_rekey_period"] = strconv.Itoa(c.rekeyPeriod)
	return ret, nil
}

// ShillServiceProperties returns shill properties of DynamicWEP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret, err := c.Config.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill properties from underlying EAP Config")
	}
	ret[shill.ServicePropertyEAPKeyMgmt] = shill.ServiceKeyMgmtIEEE8021X
	return ret, nil
}

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	blueprint    Config
	serverCACert string
	serverCred   certificate.Credential
	eapOps       []eap.Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert string, serverCred certificate.Credential, ops ...Option) *ConfigFactory {
	fac := &ConfigFactory{
		// Default config.
		blueprint: Config{
			rekeyPeriod: defaultRekeyPeriod,
			useShortKey: false,
		},
		serverCACert: serverCACert,
		serverCred:   serverCred,
	}
	for _, op := range ops {
		op(fac)
	}
	return fac
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	eapConf, err := eap.NewConfigFactory(f.serverCACert, f.serverCred, f.eapOps...).Gen()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build underlying EAP Config")
	}

	// conf's type is dynamicwep.Config, which embeds an eap.Config that is obtained above.
	// The reason why it stores blueprint, a partially initialized object, in ConfigFactory is that
	// we can use it to carry fields that don't raise error when being assigned.
	// For those fields may raise error during initialization, postpone their assignment to here.
	conf := f.blueprint
	conf.Config = eapConf.(*eap.Config)

	return &conf, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
