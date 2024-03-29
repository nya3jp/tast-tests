// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	testUser        = "test-user"
	testPassword    = "test-password"
	testPackageName = "app.passpoint.example.com"
)

var testCerts = certificate.TestCert1()

// Auth is the authentication method the access point will expose.
type Auth int

const (
	// AuthTLS represents the EAP-TLS authentication method.
	AuthTLS Auth = iota
	// AuthTTLS represents the EAP-TTLS with MSCHAPv2 authentication method.
	AuthTTLS
)

// Credentials represents a set of Passpoint credentials with selection criteria.
type Credentials struct {
	// Domains represents the domains of the compatible service providers.
	// The first domain is the FQDN of the provider, the others (if any)
	// are the FQDNs of partner service providers.
	Domains []string
	// HomeOIs is a list of organisation identifiers (OI).
	HomeOIs []uint64
	// RequiredHomeOIs is a list of required organisation identifiers.
	RequiredHomeOIs []uint64
	// RoamingOIs is a list of roaming-compatible OIs.
	RoamingOIs []uint64
	// Auth is the EAP network authentication.
	Auth
}

// FQDN returns the fully qualified domain name of the service provider.
// The first domain of the pc.Domains list is considered to be the FQDN
// that identifies the service provider (ARC follows the same logic).
func (pc *Credentials) FQDN() string {
	if len(pc.Domains) == 0 {
		return ""
	}
	return pc.Domains[0]
}

// OtherHomePartners returns the list compatible partners' domains.
func (pc *Credentials) OtherHomePartners() []string {
	if len(pc.Domains) < 2 {
		return []string{}
	}
	return pc.Domains[1:]
}

// ToShillProperties converts the set of credentials to a map for credentials D-Bus
// properties. ToShillProperties only supports EAP-TTLS authentication.
func (pc *Credentials) ToShillProperties() (map[string]interface{}, error) {
	if pc.Auth != AuthTTLS {
		return nil, errors.Errorf("unsupported authentication method: %v", pc.Auth)
	}

	props := map[string]interface{}{
		shillconst.PasspointCredentialsPropertyDomains:            pc.Domains,
		shillconst.PasspointCredentialsPropertyRealm:              pc.FQDN(),
		shillconst.PasspointCredentialsPropertyMeteredOverride:    false,
		shillconst.PasspointCredentialsPropertyAndroidPackageName: testPackageName,
		shillconst.ServicePropertyEAPMethod:                       "TTLS",
		shillconst.ServicePropertyEAPInnerEAP:                     "auth=MSCHAPV2",
		shillconst.ServicePropertyEAPIdentity:                     testUser,
		shillconst.ServicePropertyEAPPassword:                     testPassword,
		shillconst.ServicePropertyEAPCACertPEM:                    []string{testCerts.CACred.Cert},
	}

	for propName, ois := range map[string][]uint64{
		shillconst.PasspointCredentialsPropertyHomeOIs:          pc.HomeOIs,
		shillconst.PasspointCredentialsPropertyRequiredHomeOIs:  pc.RequiredHomeOIs,
		shillconst.PasspointCredentialsPropertyRoamingConsortia: pc.RoamingOIs,
	} {
		var propOIs []string
		for _, oi := range ois {
			propOIs = append(propOIs, strconv.FormatUint(oi, 10))
		}
		props[propName] = propOIs
	}

	return props, nil
}

// ToAndroidConfig converts the set of credentials to a base64 string of an Android-specific type application/x-wifi-config.
func (pc *Credentials) ToAndroidConfig(ctx context.Context) (string, error) {
	ppsMoProfile, err := pc.preparePPSMOProfile()
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare OMA-DM PerProviderSubscription-MO XML profile")
	}

	params := map[string]string{
		"profile": base64.StdEncoding.EncodeToString([]byte(ppsMoProfile)),
		"caCert":  base64.StdEncoding.EncodeToString([]byte(testCerts.CACred.Cert)),
	}

	if pc.Auth == AuthTLS {
		pkcs12Cert, err := preparePKCS12Cert(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to create PKCS#12 certificate")
		}
		params["clientCert"] = base64.StdEncoding.EncodeToString([]byte(pkcs12Cert))
	}

	tmpl, err := template.New("").Parse(`Content-Type: multipart/mixed; boundary={boundary}
Content-Transfer-Encoding: base64

--{boundary}
Content-Type: application/x-passpoint-profile
Content-Transfer-Encoding: base64

{{.profile}}
--{boundary}
Content-Type: application/x-x509-ca-cert
Content-Transfer-Encoding: base64

{{.caCert}}
{{- if .clientCert}}
--{boundary}
Content-Type: application/x-pkcs12
Content-Transfer-Encoding: base64

{{.clientCert}}
{{- end}}
--{boundary}--`)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse android config template")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", errors.Wrap(err, "failed to execute android config template")
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// preparePPSMOProfile creates an OMA-DM PerProviderSubscription-MO XML profile based on the set of Passpoint credentials.
func (pc *Credentials) preparePPSMOProfile() (string, error) {
	homeSp, err := pc.preparePPSMOHomeSP()
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare OMA-DM PerProviderSubscription Home SP node")
	}
	cred, err := pc.preparePPSMOCred()
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare OMA-DM PerProviderSubscription-MO credential node")
	}
	return fmt.Sprintf(`<MgmtTree xmlns="syncml:dmddf1.2">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>PerProviderSubscription</NodeName>
    <RTProperties>
      <Type>
        <DDFName>urn:wfa:mo:hotspot2dot0-perprovidersubscription:1.0</DDFName>
      </Type>
    </RTProperties>
    <Node>
      <NodeName>i001</NodeName>
%s
%s
    </Node>
  </Node>
</MgmtTree>`, homeSp, cred), nil
}

// preparePPSMOHomeSP creates an OMA-DM PerProviderSubscription-MO XML Home SP node based on the set of Passpoint credentials.
func (pc *Credentials) preparePPSMOHomeSP() (string, error) {
	type homeOI struct {
		Value    string
		Required string
	}
	var ois []homeOI
	for _, oi := range pc.HomeOIs {
		ois = append(ois, homeOI{
			Value:    strconv.FormatUint(oi, 16),
			Required: "FALSE",
		})
	}
	for _, oi := range pc.RequiredHomeOIs {
		ois = append(ois, homeOI{
			Value:    strconv.FormatUint(oi, 16),
			Required: "TRUE",
		})
	}
	var roamingOIs []string
	for _, oi := range pc.RoamingOIs {
		roamingOIs = append(roamingOIs, strconv.FormatUint(oi, 16))
	}

	// Nodes are starting at index 1, we need a helper to fill the template.
	funcs := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}

	tmpl, err := template.New("").Funcs(funcs).Parse(`
      <Node>
        <NodeName>HomeSP</NodeName>
        <Node>
          <NodeName>FriendlyName</NodeName>
          <Value>Android Passpoint Config</Value>
        </Node>
        <Node>
          <NodeName>FQDN</NodeName>
          <Value>{{.FQDN}}</Value>
        </Node>
{{- if gt (len .HomeOIs) 0}}
        <Node>
          <NodeName>HomeOIList</NodeName>
{{- range $i, $oi := .HomeOIs}}
          <Node>
            <NodeName>h{{printf "%03d" (inc $i)}}</NodeName>
            <Node>
              <NodeName>HomeOI</NodeName>
              <Value>{{$oi.Value}}</Value>
            </Node>
            <Node>
              <NodeName>HomeOIRequired</NodeName>
              <Value>{{$oi.Required}}</Value>
            </Node>
          </Node>
{{- end}}
        </Node>
{{- end}}
{{- if gt (len .RoamingOIs) 0}}
        <Node>
          <NodeName>RoamingConsortiumOI</NodeName>
          <Value>{{.RoamingOIs}}</Value>
        </Node>
{{- end}}
{{- if gt (len .OtherHomePartners) 0}}
        <Node>
          <NodeName>OtherHomePartners</NodeName>
{{- range $i, $fqdn := .OtherHomePartners}}
          <Node>
            <NodeName>o{{printf "%03d" (inc $i)}}</NodeName>
            <Node>
              <NodeName>FQDN</NodeName>
              <Value>{{$fqdn}}</Value>
            </Node>
          </Node>
{{- end}}
        </Node>
{{- end}}
      </Node>
	`)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse PPS Home SP template")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		FQDN              string
		HomeOIs           []homeOI
		RoamingOIs        string
		OtherHomePartners []string
	}{
		FQDN:              pc.FQDN(),
		HomeOIs:           ois,
		RoamingOIs:        strings.Join(roamingOIs, ","),
		OtherHomePartners: pc.OtherHomePartners(),
	}); err != nil {
		return "", errors.Wrap(err, "failed to execute PPS Home SP template")
	}
	return buf.String(), nil
}

// preparePPSMOCred creates an OMA-DM PerProviderSubscription-MO XML Credential node based on the set of Passpoint credentials.
func (pc *Credentials) preparePPSMOCred() (string, error) {
	params := map[string]string{
		"realm": pc.FQDN(),
	}
	switch pc.Auth {
	case AuthTLS:
		fingerprint, err := prepareCertSHA256Fingerprint()
		if err != nil {
			return "", errors.Wrap(err, "failed to get certificate's fingerprint")
		}
		params["fingerprint"] = fingerprint
	case AuthTTLS:
		params["user"] = testUser
		params["password"] = base64.StdEncoding.EncodeToString([]byte(testPassword))
	default:
		return "", errors.Errorf("unsupported authentication method: %v", pc.Auth)
	}

	tmpl, err := template.New("").Parse(`
      <Node>
        <NodeName>Credential</NodeName>
        <Node>
          <NodeName>Realm</NodeName>
          <Value>{{.realm}}</Value>
        </Node>
{{if .fingerprint}}
        <Node>
          <NodeName>DigitalCertificate</NodeName>
          <Node>
            <NodeName>CertificateType</NodeName>
            <Value>x509v3</Value>
          </Node>
          <Node>
            <NodeName>CertSHA256Fingerprint</NodeName>
            <Value>{{.fingerprint}}</Value>
          </Node>
        </Node>
{{else if and .user .password}}
        <Node>
          <NodeName>UsernamePassword</NodeName>
          <Node>
            <NodeName>MachineManaged</NodeName>
            <Value>true</Value>
          </Node>
          <Node>
            <NodeName>Username</NodeName>
            <Value>{{.user}}</Value>
          </Node>
          <Node>
            <NodeName>Password</NodeName>
            <Value>{{.password}}</Value>
          </Node>
          <Node>
            <NodeName>EAPMethod</NodeName>
            <Node>
              <NodeName>EAPType</NodeName>
              <Value>21</Value>
            </Node>
            <Node>
              <NodeName>InnerMethod</NodeName>
              <Value>MS-CHAP-V2</Value>
            </Node>
          </Node>
        </Node>
{{end}}
      </Node>`)

	if err != nil {
		return "", errors.Wrap(err, "failed to parse credentials template")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", errors.Wrap(err, "failed to execute credentials template")
	}
	return buf.String(), nil
}

// prepareCertSHA256Fingerprint gets the client certificate's SHA256 fingerprint.
func prepareCertSHA256Fingerprint() (string, error) {
	block, _ := pem.Decode([]byte(testCerts.ClientCred.Cert))
	if block == nil {
		return "", errors.New("failed to parse PEM file")
	}
	hash := sha256.New()
	hash.Write(block.Bytes)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// preparePKCS12Cert create a PKCS#12 format certificate from its client's certificate and private key.
func preparePKCS12Cert(ctx context.Context) (cert string, retErr error) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary directory")
	}
	defer func() {
		if retErr != nil {
			if err := os.RemoveAll(tmpDir); err != nil {
				testing.ContextLogf(ctx, "Failed to clean up dir %s, %v", tmpDir, err)
			}
		}
	}()

	clientCertPath := filepath.Join(tmpDir, "cert")
	privateKeyPath := filepath.Join(tmpDir, "private_key")

	for _, p := range []struct {
		path     string
		contents string
	}{
		{clientCertPath, testCerts.ClientCred.Cert},
		{privateKeyPath, testCerts.ClientCred.PrivateKey},
	} {
		if err := ioutil.WriteFile(p.path, []byte(p.contents), 0644); err != nil {
			return "", errors.Wrapf(err, "failed to write file %q", p.path)
		}
	}
	out, err := testexec.CommandContext(ctx, "openssl", "pkcs12", "-export", "-inkey", privateKeyPath, "-in", clientCertPath, "-password", "pass:").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to create PKCS#12 certificate")
	}
	return string(out), nil
}
