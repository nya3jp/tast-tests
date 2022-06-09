// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hostapd"
)

// Auth is the authentication method the access point will expose.
type Auth int

const (
	// AuthTLS represents the EAP-TLS authentication method.
	AuthTLS Auth = iota
	// AuthTTLS represents the EAP-TTLS with MSCHAPv2 authentication method.
	AuthTTLS
)

// AccessPoint describes a Passpoint compatible access point with its match criteria.
type AccessPoint struct {
	// SSID is the name of the network.
	SSID string
	// Domain is the FQDN of the provider of this network.
	Domain string
	// Realms is the set of FQDN supported by this network.
	Realms []string
	// RoamingConsortium is the OI supported by this network.
	// TODO(b/232747458): support multiple roaming consortium OIs.
	RoamingConsortium uint64
	// Auth is the EAP network authentication.
	Auth
}

// ToServer transforms the Passpoint access point descriptions into a valid hostapd service instance.
func (ap *AccessPoint) ToServer(iface, outDir string) *hostapd.Server {
	certs := certificate.TestCert1()
	return hostapd.NewServer(
		iface,
		filepath.Join(outDir, iface),
		NewAPConf(
			ap.SSID,
			ap.Auth,
			testUser,
			testPassword,
			&certs,
			ap.Domain,
			ap.Realms,
			fmt.Sprintf("%x", ap.RoamingConsortium),
		),
	)
}

// APConf contains the parameters required to setup a Passpoint-
// compatible access point.
type APConf struct {
	// ssid is the name of the access point.
	ssid string
	// auth is the authentication method exposed by the access point.
	auth Auth
	// identity is the username of the EAP user (EAP-TTLS).
	identity string
	// password is the secret of the EAP user (EAP-TTLS).
	password string
	// cert is the set of certificates used by the radius server to prove its
	// identity and authenticate the user (EAP-TLS).
	cert *certificate.CertStore
	// domain is the FQDN of the Passpoint service provider.
	domain string
	// realms is the set of domains supported by the access point.
	realms []string
	// roamingConsortium is the Organisation Identifier (OI) of compatible networks.
	roamingConsortium string
}

// NewAPConf creates a new Passpoint compatible access point configuration from the parameters.
func NewAPConf(ssid string, auth Auth, identity, password string, cert *certificate.CertStore, domain string, realms []string, roamingConsortium string) *APConf {
	return &APConf{
		ssid:              ssid,
		auth:              auth,
		identity:          identity,
		password:          password,
		cert:              cert,
		domain:            domain,
		realms:            realms,
		roamingConsortium: roamingConsortium,
	}
}

// Generate transforms the configuration parameters in a set of configuration
// files suitable for hostapd.
func (c APConf) Generate(ctx context.Context, dir, ctrlPath string) (string, error) {
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
`, ctrlPath, c.ssid, caCertPath, serverCertPath, privateKeyPath, eapUserFilePath, c.domain, c.prepareRealms(), c.roamingConsortium)

	for _, p := range []struct {
		path     string
		contents string
	}{
		{confPath, confContents},
		{serverCertPath, c.cert.ServerCred.Cert},
		{privateKeyPath, c.cert.ServerCred.PrivateKey},
		{eapUserFilePath, eapUsers},
		{caCertPath, c.cert.CACred.Cert},
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
	switch c.auth {
	case AuthTLS:
		// TLS auth only requires an outer authentication
		return `# Outer authentication
* TLS`, nil
	case AuthTTLS:
		// TTLS requires outer and inner authentication
		return fmt.Sprintf(`# Outer authentication
* TTLS
# Inner authentication
"%s" TTLS-MSCHAPV2 "%s" [2]`, c.identity, c.password), nil
	default:
		return "", errors.Errorf("unsupported authentication method: %v", c.auth)
	}
}

// prepareRealms creates the list of realm domain names with the correct
// authentication parameters.
func (c APConf) prepareRealms() string {
	realms := c.domain
	if len(c.realms) > 0 {
		realms = strings.Join(c.realms, ";")
	}

	switch c.auth {
	case AuthTLS:
		// EAP method TLS (13) with credentials type (5) set to certificate (6).
		return fmt.Sprintf("%s,13[5:6]", realms)
	case AuthTTLS:
		// EAP method TTLS (21) with inner authentication (2) set to
		// MSCHAPV2 (4) and credentials type (5) set to username/password (7).
		return fmt.Sprintf("%s,21[2:4][5:7]", realms)
	default:
		return realms
	}
}

// WaitForSTAAssociated polls an access point until a specific station is
// associated or timeout is fired.
func WaitForSTAAssociated(ctx context.Context, m *hostapd.Monitor, client string, timeout time.Duration) error {
	timeoutContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	iface, err := net.InterfaceByName(client)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain %s interface information: ", client)
	}
	addr := iface.HardwareAddr.String()

	for {
		event, err := m.WaitForEvent(timeoutContext)
		if err != nil {
			return errors.Wrap(err, "failed to wait for AP event")
		}
		if event == nil { // timeout
			return errors.New("association event timeout")
		}
		switch e := event.(type) {
		case *hostapd.ApStaConnectedEvent:
			if e.Addr != addr {
				return errors.Errorf("unexpected station association: expecting %s got %s", addr, e.Addr)
			}
			return nil
		default:
		}
	}
}
