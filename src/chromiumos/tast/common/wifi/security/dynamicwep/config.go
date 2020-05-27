// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dynamicwep provides a Config type for Dynamic WEP protected network.
package dynamicwep

import (
	"strconv"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// DefaultRekeyPeriod is the default rekey period in second of Dynamic WEP.
const DefaultRekeyPeriod = 20

// Config implements security.Config interface for Dynamic WEP protected network.
type Config struct {
	// Embedded EAP Config to inherit the Install* methods.
	*eap.Config

	useShortKey bool
	rekeyPeriod int
}

// Class returns security class of EAP network.
func (c *Config) Class() string {
	return shill.SecurityWEP
}

// HostapdConfig returns hostapd config of EAP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	ret, err := c.Config.HostapdConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hostapd config from underlying EAP Config")
	}
	keyLen := 13
	if c.useShortKey {
		keyLen = 5
	}
	ret["wep_key_len_broadcast"] = strconv.Itoa(keyLen)
	ret["wep_key_len_unicast"] = strconv.Itoa(keyLen)
	ret["wep_rekey_period"] = strconv.Itoa(c.rekeyPeriod)
	return ret, nil
}

// ShillServiceProperties returns shill properties of EAP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret, err := c.Config.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill properties from underlying EAP Config")
	}
	ret[shill.ServicePropertyEAPKeyMgmt] = shill.ServicePropertyEAPKeyMgmtIEEE8021X
	return ret, nil
}

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	blueprint    *Config
	serverCACert string
	serverCert   string
	serverKey    string
	eapOps       []eap.Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert, serverCert, serverKey string, ops ...Option) *ConfigFactory {
	fac := &ConfigFactory{
		// Default config.
		blueprint: &Config{
			rekeyPeriod: DefaultRekeyPeriod,
			useShortKey: false,
		},
		serverCACert: serverCACert,
		serverCert:   serverCert,
		serverKey:    serverKey,
	}
	for _, op := range ops {
		op(fac)
	}
	return fac
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	eapConf, err := eap.NewConfigFactory(f.serverCACert, f.serverCert, f.serverKey, f.eapOps...).Gen()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build underlying EAP Config")
	}

	conf := *f.blueprint
	conf.Config = eapConf.(*eap.Config)

	return &conf, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
