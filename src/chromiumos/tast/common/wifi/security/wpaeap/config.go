// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpaeap provides a Config type for WPA EAP protected network.
package wpaeap

import (
	"strconv"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// Config implements security.Config interface for WPA-EAP protected network.
type Config struct {
	// Embedded EAP Config to inherit the Install* methods.
	*eap.Config

	mode            wpa.ModeEnum
	ftMode          wpa.FTModeEnum
	useSystemCAs    bool
	altSubjectMatch []string
}

// HostapdConfig returns hostapd config of WPA-EAP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	ret, err := c.Config.HostapdConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hostapd config from underlying EAP Config")
	}

	ret["wpa"] = strconv.Itoa(int(c.mode))
	ret["wpa_pairwise"] = "CCMP"

	switch c.ftMode {
	case wpa.FTModeNone:
		ret["wpa_key_mgmt"] = "WPA-EAP"
	case wpa.FTModePure:
		ret["wpa_key_mgmt"] = "FT-EAP"
	case wpa.FTModeMixed:
		ret["wpa_key_mgmt"] = "WPA-EAP FT-EAP"
	default:
		return nil, errors.Errorf("invalid ftMode %d", c.ftMode)
	}

	return ret, nil
}

// ShillServiceProperties returns shill properties of WPA-EAP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret, err := c.Config.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill properties from underlying EAP Config")
	}

	ret[shill.ServicePropertyEAPUseSystemCAs] = c.useSystemCAs
	if c.ftMode&wpa.FTModePure > 0 {
		ret[shill.ServicePropertyFTEnabled] = true
	}
	if len(c.altSubjectMatch) > 0 {
		ret[shill.ServicePropertyEAPSubjectAlternativeNameMatch] = append([]string(nil), c.altSubjectMatch...)
	}

	return ret, nil
}

func (c *Config) validate() error {
	if c.mode&(^wpa.ModeMixed) > 0 || c.mode == 0 {
		return errors.Errorf("invalid mode %d", c.mode)
	}
	if c.ftMode&(^wpa.FTModeMixed) > 0 || c.ftMode == 0 {
		return errors.Errorf("invalid ftMode %d", c.ftMode)
	}
	return nil
}

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	blueprint    Config
	serverCACert string
	serverCert   string
	serverKey    string
	eapOps       []eap.Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert, serverCert, serverKey string, ops ...Option) *ConfigFactory {
	fac := &ConfigFactory{
		// Default config.
		blueprint: Config{
			mode:         wpa.ModePureWPA,
			ftMode:       wpa.FTModeNone,
			useSystemCAs: true,
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

	conf := f.blueprint
	conf.altSubjectMatch = append([]string(nil), f.blueprint.altSubjectMatch...)
	conf.Config = eapConf.(*eap.Config)

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return &conf, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
