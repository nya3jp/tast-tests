// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package passpoint implements a library that provides common tools for
// Passpoint tests.
package passpoint

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
)

// Auth is the authentication method the access point will expose.
type Auth int

const (
	// AuthTLS represents the EAP-TLS authentication method.
	AuthTLS Auth = iota
	// AuthTTLS represents the EAP-TTLS with MSCHAPv2 authentication method.
	AuthTTLS
)

// APConf contains the parameters required to setup a Passpoint
// compatible access point.
type APConf struct {
	// Access point network name.
	SSID string
	// Authentication method exposed by the access point.
	Auth
	// Username of the EAP user.
	Identity string
	// Password of the EAP user (EAP-TTLS only).
	Password string
	// Certificate set used by the radius server to prove its identity and
	// authentify the user (EAP-TLS only).
	Cert *certificate.CertStore
	// FQDN of the Passpoint service provider.
	Domain string
	// Set of realms (domains) supported by the access point.
	Realms []string
	// Organisation identifier (OI) of compatible networks.
	RoamingConsortium string
}

// Prepare transforms the configuration parameters in a set of configuration
// files suitable for hostapd.
func (c APConf) Prepare(ctx context.Context, dir, ctrlPath string) (string, error) {
	serverCertPath := filepath.Join(dir, "cert")
	privateKeyPath := filepath.Join(dir, "private_key")
	eapUserFilePath := filepath.Join(dir, "eap_user")
	caCertPath := filepath.Join(dir, "ca_cert")
	confPath := filepath.Join(dir, "hostapd.conf")

	// Create the radius users configuration.
	eapUsers, err := c.prepareEAPUsers()
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare EAP users file")
	}

	confContents := fmt.Sprintf(`ctrl_interface=%s
# Wireless configuration
ssid=%s
hw_mode=g
channel=1
# Enable EAP authentication and server
ieee8021x=1
eapol_version=2
eap_server=1
ca_cert=%s
server_cert=%s
private_key=%s
eap_user_file=%s
# Security
wpa=2
wpa_key_mgmt=WPA-EAP WPA-EAP-SHA256
wpa_pairwise=CCMP
ieee80211w=1
# Interworking (802.11u-2011)
interworking=1
domain_name=%s
nai_realm=0,%s
roaming_consortium=%s
# Hotspot 2.0
hs20=1
`, ctrlPath, c.SSID, caCertPath, serverCertPath, privateKeyPath, eapUserFilePath, c.Domain, c.prepareRealms(), c.RoamingConsortium)

	for _, p := range []struct {
		path     string
		contents string
	}{
		{confPath, confContents},
		{serverCertPath, c.Cert.ServerCred.Cert},
		{privateKeyPath, c.Cert.ServerCred.PrivateKey},
		{eapUserFilePath, eapUsers},
		{caCertPath, c.Cert.CACred.Cert},
	} {
		if err := ioutil.WriteFile(p.path, []byte(p.contents), 0644); err != nil {
			return "", errors.Wrapf(err, "failed to write file %q", p.path)
		}
	}

	return confPath, nil
}

// prepareEAPUsers creates the content of the radius users file that describes
// how to authenticate users.
func (c APConf) prepareEAPUsers() (string, error) {
	switch c.Auth {
	case AuthTLS:
		// TLS auth only requires an outer authentication
		return `# Outer authentication
* TLS`, nil
	case AuthTTLS:
		// TTLS requires outer and inner authentication
		return fmt.Sprintf(`# Outer authentication
* TTLS
# Inner authentication
"%s" TTLS-MSCHAPV2 "%s" [2]`, c.Identity, c.Password), nil
	}

	return "", errors.Errorf("unsupported authentication method: %v", c.Auth)
}

// prepareRealms creates the list of realm domain names with the correct
// authentication parameters.
func (c APConf) prepareRealms() string {
	realms := c.Domain
	if len(c.Realms) > 0 {
		realms = strings.Join(c.Realms, ";")
	}

	if c.Auth == AuthTLS {
		// EAP method TLS (13) with credentials type (5) set to certificate (6).
		return fmt.Sprintf("%s,13[5:6]", realms)
	}

	if c.Auth == AuthTTLS {
		// EAP method TTLS (21) with inner authentication (2) set to
		// MSCHAPV2 (4) and credentials type (5) set to username/password (7).
		return fmt.Sprintf("%s,21[2:4][5:7]", realms)
	}

	return realms
}
