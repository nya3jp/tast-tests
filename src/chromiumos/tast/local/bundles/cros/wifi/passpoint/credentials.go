// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
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
	// Domain represents the domain of the service provider.
	Domain string
	// HomeOI is a list of organisation identifiers (OI).
	HomeOIs []uint64
	// RequiredHomeOIs is a list of required organisation identifiers.
	RequiredHomeOIs []uint64
	// RoamingOIs is a list of roaming compatible OIs.
	RoamingOIs []uint64
	// Auth is the EAP network authentication.
	Auth
}

// ToShillProperties converts the set of credentials to a map for credentials D-Bus
// properties. ToShillProperties only supports EAP-TTLS authentication.
func (pc *Credentials) ToShillProperties() (map[string]interface{}, error) {
	if pc.Auth != AuthTTLS {
		return nil, errors.Errorf("unsupported authentication method: %v", pc.Auth)
	}

	props := map[string]interface{}{
		shillconst.PasspointCredentialsPropertyDomains:            []string{pc.Domain},
		shillconst.PasspointCredentialsPropertyRealm:              pc.Domain,
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
	androidConfig := fmt.Sprintf(`Content-Type: multipart/mixed; boundary={boundary}
Content-Transfer-Encoding: base64

--{boundary}
Content-Type: application/x-passpoint-profile
Content-Transfer-Encoding: base64

%s
--{boundary}
Content-Type: application/x-x509-ca-cert
Content-Transfer-Encoding: base64

%s
`, base64.StdEncoding.EncodeToString([]byte(ppsMoProfile)), base64.StdEncoding.EncodeToString([]byte(testCerts.CACred.Cert)))

	if pc.Auth == AuthTLS {
		pkcs12Cert, err := preparePKCS12Cert(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to create PKCS#12 certificate")
		}
		androidConfig += fmt.Sprintf(`--{boundary}
Content-Type: application/x-pkcs12
Content-Transfer-Encoding: base64

%s
`, base64.StdEncoding.EncodeToString([]byte(pkcs12Cert)))
	}
	androidConfig += "--{boundary}--"
	return base64.StdEncoding.EncodeToString([]byte(androidConfig)), nil
}

// preparePPSMOProfile creates an OMA-DM PerProviderSubscription-MO XML profile based on the set of Passpoint credentials.
func (pc *Credentials) preparePPSMOProfile() (string, error) {
	homeSp := pc.preparePPSMOHomeSP()
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
func (pc *Credentials) preparePPSMOHomeSP() string {
	type homeOI struct {
		oi       string
		required string
	}
	var ois []homeOI
	for _, homeOi := range pc.HomeOIs {
		ois = append(ois, homeOI{oi: strconv.FormatUint(homeOi, 16), required: "FALSE"})
	}
	for _, homeOi := range pc.RequiredHomeOIs {
		ois = append(ois, homeOI{oi: strconv.FormatUint(homeOi, 16), required: "TRUE"})
	}

	// Create Home OI node(s).
	var homeOis string
	if len(ois) > 0 {
		var homeOiNodes string
		for i, oi := range ois {
			homeOiNodes += fmt.Sprintf(`          <Node>
            <NodeName>x%d</NodeName>
            <Node>
              <NodeName>HomeOI</NodeName>
              <Value>%s</Value>
            </Node>
            <Node>
              <NodeName>HomeOIRequired</NodeName>
              <Value>%s</Value>
            </Node>
          </Node>
`, i+1, oi.oi, oi.required)
		}
		homeOis = fmt.Sprintf(`        <Node>
          <NodeName>HomeOIList</NodeName>
%s        </Node>`, homeOiNodes)
	}

	// Create Roaming OI node.
	var roamingOis string
	if len(pc.RoamingOIs) > 0 {
		var roamingOiValues []string
		for _, oi := range pc.RoamingOIs {
			roamingOiValues = append(roamingOiValues, strconv.FormatUint(oi, 16))
		}
		roamingOis = fmt.Sprintf(`        <Node>
          <NodeName>RoamingConsortiumOI</NodeName>
          <Value>%s</Value>
        </Node>`, strings.Join(roamingOiValues, ","))
	}

	// Combine the OI(s) into a Home SP node.
	return fmt.Sprintf(`      <Node>
        <NodeName>HomeSP</NodeName>
        <Node>
          <NodeName>FriendlyName</NodeName>
          <Value>Android Passpoint Config</Value>
        </Node>
        <Node>
          <NodeName>FQDN</NodeName>
          <Value>%s</Value>
        </Node>
%s
%s
      </Node>`, pc.Domain, homeOis, roamingOis)
}

// preparePPSMOCred creates an OMA-DM PerProviderSubscription-MO XML Credential node based on the set of Passpoint credentials.
func (pc *Credentials) preparePPSMOCred() (string, error) {
	switch pc.Auth {
	case AuthTLS:
		fingerprint, err := prepareCertSHA256Fingerprint()
		if err != nil {
			return "", errors.Wrap(err, "failed to get certificate's fingerprint")
		}
		return fmt.Sprintf(`      <Node>
        <NodeName>Credential</NodeName>
        <Node>
          <NodeName>Realm</NodeName>
          <Value>%s</Value>
        </Node>
        <Node>
          <NodeName>DigitalCertificate</NodeName>
          <Node>
            <NodeName>CertificateType</NodeName>
            <Value>x509v3</Value>
          </Node>
          <Node>
            <NodeName>CertSHA256Fingerprint</NodeName>
            <Value>%s</Value>
          </Node>
        </Node>
      </Node>`, pc.Domain, fingerprint), nil
	case AuthTTLS:
		return fmt.Sprintf(`      <Node>
      <NodeName>Credential</NodeName>
      <Node>
        <NodeName>Realm</NodeName>
        <Value>%s</Value>
      </Node>
      <Node>
        <NodeName>UsernamePassword</NodeName>
        <Node>
          <NodeName>MachineManaged</NodeName>
          <Value>true</Value>
        </Node>
        <Node>
          <NodeName>Username</NodeName>
          <Value>%s</Value>
        </Node>
        <Node>
          <NodeName>Password</NodeName>
          <Value>%s</Value>
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
    </Node>`, pc.Domain, testUser, base64.StdEncoding.EncodeToString([]byte(testPassword))), nil
	default:
		return "", errors.Errorf("unsupported authentication method: %v", pc.Auth)
	}
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
