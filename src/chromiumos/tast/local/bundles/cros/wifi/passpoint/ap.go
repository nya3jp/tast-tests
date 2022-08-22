// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hostapd"
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
	RoamingConsortiums []uint64
	// Auth is the EAP network authentication.
	Auth
}

// ToServer transforms the Passpoint access point descriptions into a valid hostapd service instance.
func (ap *AccessPoint) ToServer(iface, outDir string) *hostapd.Server {
	certs := certificate.TestCert1()
	var roamingConsortiums []string
	for _, rc := range ap.RoamingConsortiums {
		roamingConsortiums = append(roamingConsortiums, fmt.Sprintf("%x", rc))
	}
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
			roamingConsortiums,
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
	// roamingConsortiums is the Organisation Identifier (OI) list of compatible networks.
	roamingConsortiums []string
}

// NewAPConf creates a new Passpoint compatible access point configuration from the parameters.
func NewAPConf(ssid string, auth Auth, identity, password string, cert *certificate.CertStore, domain string, realms, roamingConsortiums []string) *APConf {
	return &APConf{
		ssid:               ssid,
		auth:               auth,
		identity:           identity,
		password:           password,
		cert:               cert,
		domain:             domain,
		realms:             realms,
		roamingConsortiums: roamingConsortiums,
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

	confContents, err := c.prepareConf(ctrlPath, caCertPath, serverCertPath, privateKeyPath, eapUserFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare configuration file")
	}

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

// prepareConf generates the content of hostapd configuration file.
func (c APConf) prepareConf(socketPath, caPath, certPath, keyPath, eapUsers string) (string, error) {
	tmpl := template.Must(template.New("").Parse(`
ctrl_interface={{.CtrlSocket}}
# Wireless configuration
ssid={{.SSID}}
hw_mode=g
channel=1
# Enable EAP authentication and server
ieee8021x=1
eapol_version=2
eap_server=1
ca_cert={{.CaCert}}
server_cert={{.ServerCert}}
private_key={{.PrivateKey}}
eap_user_file={{.EapUsers}}
# Security
wpa=2
wpa_key_mgmt=WPA-EAP WPA-EAP-SHA256
wpa_pairwise=CCMP
ieee80211w=1
# Interworking (802.11u-2011)
interworking=1
domain_name={{.Domains}}
nai_realm=0,{{.Realms}}
{{range .RoamingConsortiums}}
roaming_consortium={{.}}
{{end}}
# Hotspot 2.0
hs20=1
`))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		CtrlSocket         string
		SSID               string
		CaCert             string
		ServerCert         string
		PrivateKey         string
		EapUsers           string
		Domains            string
		Realms             string
		RoamingConsortiums []string
	}{
		CtrlSocket:         socketPath,
		SSID:               c.ssid,
		CaCert:             caPath,
		ServerCert:         certPath,
		PrivateKey:         keyPath,
		EapUsers:           eapUsers,
		Domains:            c.domain,
		Realms:             c.prepareRealms(),
		RoamingConsortiums: c.roamingConsortiums,
	}); err != nil {
		return "", errors.Wrap(err, "failed to execute hostapd configuration template")
	}
	return buf.String(), nil
}

// STAAssociationTimeout is the reasonable delay to wait for station
// association before making a decision.
const STAAssociationTimeout = time.Minute

// WaitForSTAAssociated polls an access point until a specific station is
// associated or timeout is fired.
func WaitForSTAAssociated(ctx context.Context, m *hostapd.Monitor, client string, timeout time.Duration) error {
	return waitForSTAAssociationEvent(ctx, m, client, timeout, true)
}

// WaitForSTADissociated polls an access point until a specific station is
// dissociated or timeout is fired.
func WaitForSTADissociated(ctx context.Context, m *hostapd.Monitor, client string, timeout time.Duration) error {
	return waitForSTAAssociationEvent(ctx, m, client, timeout, false)
}

// waitForSTAAssociationEvent polls an access point for until a specific station got an
// association event (association or dissociation based on the parameter association) or until timeout is fired.
func waitForSTAAssociationEvent(ctx context.Context, m *hostapd.Monitor, client string, timeout time.Duration, association bool) error {
	timeoutContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	iface, err := net.InterfaceByName(client)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain %s interface information: ", client)
	}

	for {
		event, err := m.WaitForEvent(timeoutContext)
		if err != nil {
			return errors.Wrap(err, "failed to wait for AP event")
		}
		if event == nil { // timeout
			return errors.New("association event timeout")
		}
		if e, ok := event.(*hostapd.ApStaConnectedEvent); ok && association {
			if bytes.Compare(iface.HardwareAddr, e.Addr) != 0 {
				return errors.Errorf("unexpected station association: got %v want %v", e.Addr, iface.HardwareAddr)
			}
			return nil
		}
		if e, ok := event.(*hostapd.ApStaDisconnectedEvent); ok && !association {
			if bytes.Compare(iface.HardwareAddr, e.Addr) != 0 {
				return errors.Errorf("unexpected station association: got %v want %v", e.Addr, iface.HardwareAddr)
			}
			return nil
		}
	}
}
