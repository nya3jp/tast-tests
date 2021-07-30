// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpaeap provides a Config type for WPA EAP protected network.
package wpaeap

import (
	"strconv"
	"strings"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/eap"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
)

// Config implements security.Config interface for WPA-EAP protected network.
type Config struct {
	// Embedded EAP Config to inherit the Install* methods.
	*eap.Config

	mode              wpa.ModeEnum
	ftMode            wpa.FTModeEnum
	useSystemCAs      bool
	altSubjectMatch   []string
	domainSuffixMatch []string
}

// HostapdConfig returns hostapd config of WPA-EAP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	ret, err := c.Config.HostapdConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hostapd config from underlying EAP Config")
	}

	var mode int
	// WPA2 and WPA3 are both RSN, and share the same bit in wpa=.
	if c.mode&(wpa.ModePureWPA2|wpa.ModePureWPA3) > 0 {
		mode |= int(wpa.ModePureWPA2)
	}
	if c.mode&wpa.ModePureWPA > 0 {
		mode |= int(wpa.ModePureWPA)
	}
	ret["wpa"] = strconv.Itoa(mode)
	ret["wpa_pairwise"] = "CCMP"

	var keyMgmt []string
	if c.ftMode&wpa.FTModeNone > 0 {
		// WPA2 does not require SHA256.
		if c.mode&wpa.ModeMixed > 0 {
			keyMgmt = append(keyMgmt, "WPA-EAP")
		}
		// WPA3 supports SHA256.
		if c.mode&wpa.ModePureWPA3 > 0 {
			keyMgmt = append(keyMgmt, "WPA-EAP-SHA256")
		}
	}
	if c.ftMode&wpa.FTModePure > 0 {
		// FT-EAP is already using SHA256, so it goes with either WPA/WPA2 or WPA3.
		keyMgmt = append(keyMgmt, "FT-EAP")
	}
	ret["wpa_key_mgmt"] = strings.Join(keyMgmt, " ")

	return ret, nil
}

// ShillServiceProperties returns shill properties of WPA-EAP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret, err := c.Config.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill properties from underlying EAP Config")
	}

	ret[shillconst.ServicePropertyEAPUseSystemCAs] = c.useSystemCAs
	if len(c.altSubjectMatch) > 0 {
		ret[shillconst.ServicePropertyEAPSubjectAlternativeNameMatch] = append([]string(nil), c.altSubjectMatch...)
	}

	if len(c.domainSuffixMatch) > 0 {
		ret[shillconst.ServicePropertyEAPDomainSuffixMatch] = append([]string(nil), c.domainSuffixMatch...)
	}

	return ret, nil
}

func (c *Config) validate() error {
	if c.mode&(^(wpa.ModeMixed|wpa.ModeMixedWPA3)) > 0 || c.mode == 0 {
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
	serverCred   certificate.Credential
	eapOps       []eap.Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert string, serverCred certificate.Credential, ops ...Option) *ConfigFactory {
	fac := &ConfigFactory{
		// Default config.
		blueprint: Config{
			mode:         wpa.ModePureWPA,
			ftMode:       wpa.FTModeNone,
			useSystemCAs: true,
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

	conf := f.blueprint
	conf.altSubjectMatch = append([]string(nil), f.blueprint.altSubjectMatch...)
	conf.domainSuffixMatch = append([]string(nil), f.blueprint.domainSuffixMatch...)
	conf.Config = eapConf.(*eap.Config)

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return &conf, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
