// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tunneled1x provides a Config type for PEAP/TTLS protected network.
// Both PEAP and TTLS are tunneled protocols which use EAP inside of a TLS
// secured tunnel. The secured tunnel is a symmetric key encryption scheme
// negotiated under the protection of a public key in the server certificate.
// Thus, we'll see server credentials in the form of certificates, but client
// credentials in the form of passwords and a CA Cert to root the trust chain.
package tunneled1x

import (
	"fmt"
	"strings"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// Outer (layer1) and inner (layer2) protocols.
const (
	TTLSPrefix = "TTLS-"

	Layer1TypePEAP = "PEAP"
	Layer1TypeTTLS = "TTLS"

	Layer2TypeGTC          = "GTC"
	Layer2TypeMSCHAPV2     = "MSCHAPV2"
	Layer2TypeMD5          = "MD5"
	Layer2TypeTTLSMSCHAPV2 = TTLSPrefix + "MSCHAPV2"
	Layer2TypeTTLSMSCHAP   = TTLSPrefix + "MSCHAP"
	Layer2TypeTTLSPAP      = TTLSPrefix + "PAP"
)

// Config implements security.Config interface for TTLS/PEAP protected network.
type Config struct {
	// Embedded WPA-EAP Config to inherit the Install* and HostapdConfig methods.
	*wpaeap.Config

	clientPassword string
	innerProtocol  string
}

// ShillServiceProperties returns shill properties of TTLS/PEAP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	ret, err := c.Config.ShillServiceProperties()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shill properties from underlying WPA-EAP Config")
	}

	ret[shill.ServicePropertyEAPPassword] = c.clientPassword
	if strings.HasPrefix(c.innerProtocol, TTLSPrefix) {
		ret[shill.ServicePropertyEAPInnerEAP] = "auth=" + c.innerProtocol[len(TTLSPrefix):]
	}

	return ret, nil
}

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	serverCACert   string
	serverCred     certificate.Credential
	clientCACert   string
	identity       string
	serverPassword string
	clientPassword string
	outerProtocol  string
	innerProtocol  string
	wpaeapOps      []wpaeap.Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert string, serverCred certificate.Credential, clientCACert, identity, serverPassword string, ops ...Option) *ConfigFactory {
	fac := &ConfigFactory{
		serverCACert:   serverCACert,
		serverCred:     serverCred,
		clientCACert:   clientCACert,
		identity:       identity,
		serverPassword: serverPassword,

		// Default config.
		outerProtocol: Layer1TypePEAP,
		innerProtocol: Layer2TypeMD5,
	}
	for _, op := range ops {
		op(fac)
	}

	// Client password may be set to a bad password for testing. Default is real (server) password.
	if fac.clientPassword == "" {
		fac.clientPassword = fac.serverPassword
	}

	serverEAPUsers := strings.Join([]string{
		fmt.Sprintf("* %s", fac.outerProtocol),
		fmt.Sprintf(`"%s" %s`, fac.identity, fac.innerProtocol),
		fmt.Sprintf(`"%s" [2]`, fac.serverPassword),
	}, "\n")
	fac.wpaeapOps = append(fac.wpaeapOps, wpaeap.ServerEAPUsers(serverEAPUsers),
		wpaeap.Identity(fac.identity), wpaeap.ClientCACert(fac.clientCACert),
	)

	return fac
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	wpaeapConf, err := wpaeap.NewConfigFactory(f.serverCACert, f.serverCred, f.wpaeapOps...).Gen()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build underlying WPA-EAP Config")
	}

	return &Config{
		Config:         wpaeapConf.(*wpaeap.Config),
		clientPassword: f.clientPassword,
		innerProtocol:  f.innerProtocol,
	}, nil
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
