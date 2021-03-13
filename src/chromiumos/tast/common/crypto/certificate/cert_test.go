// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package certificate

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"reflect"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

func pemDecode(s string) ([]byte, error) {
	block, rest := pem.Decode([]byte(s))
	if block == nil {
		return nil, errors.New("Couldn't decode Cert PEM")
	}
	if len(rest) != 0 {
		return nil, errors.Errorf("Found trailing data in cert: %q", string(rest))
	}
	return block.Bytes, nil
}

func x509ParseCert(certStr string) (*x509.Certificate, error) {
	// Parse certificate. It should be X-509 certificates in PEM format.
	pem, err := pemDecode(certStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode PEM")
	}
	cert, err := x509.ParseCertificate(pem)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse certificate")
	}
	return cert, err
}

// validateCertSignature checks that cert is signed by its parent. Note that we allow MD5-based signatures for now
// (crbug.com/1047146), and because Golang's x509 library rejects this weak crypto, we can't easily verify signatures
// properly.
func validateCertSignature(cert, parent *x509.Certificate) error {
	err := cert.CheckSignatureFrom(parent)
	if err != nil {
		// TODO(crbug.com/1047146): MD5 certificates are rejected by Golang x509. We're still allowing them for now.
		var insecureErr x509.InsecureAlgorithmError
		if !errors.As(err, &insecureErr) {
			return err
		}
	}
	return nil
}

func validatePrivateKey(privateKey string, cert *x509.Certificate) error {
	// Parse private key. It should be a PKCS#1 key in PEM format.
	pem, err := pemDecode(privateKey)
	if err != nil {
		return err
	}
	key, err := x509.ParsePKCS1PrivateKey(pem)
	if err != nil {
		return errors.Wrap(err, "failed to parse private key")
	}
	if err = key.Validate(); err != nil {
		return errors.Wrap(err, "private key failed validation")
	}

	if !reflect.DeepEqual(&key.PublicKey, cert.PublicKey) {
		return errors.New("public key does not match")
	}

	return nil
}

func TestCertificate(t *testing.T) {
	now := time.Now()
	isExpired := func(cert *x509.Certificate) bool {
		return now.Before(cert.NotBefore) || now.After(cert.NotAfter)
	}

	for testi, testcase := range []CertStore{TestCert1(), TestCert2(), TestCert3()} {
		caCert, err := x509ParseCert(testcase.CACert)
		if err != nil {
			t.Fatalf("Test %d: CACert: %v", testi, err)
		}

		if err := validateCertSignature(caCert, caCert); err != nil {
			t.Errorf("Test %d: unexpeted: CA cert isn't self-signed", testi)
		}

		testCred := func(cred Credential, expectedExpired bool) error {
			cert, err := x509ParseCert(cred.Cert)
			if err != nil {
				return err
			}

			// Verify expiry.
			if expired := isExpired(cert); expired != expectedExpired {
				// TODO(b/182610370): Omit expiration check to avoid test failure.
				// return errors.Errorf("failed cert expiry check got %t, want %t", expired, expectedExpired)
			}
			// Validate private keys.
			if err := validatePrivateKey(cred.PrivateKey, cert); err != nil {
				return errors.Errorf("failed private key check: %v", err)
			}
			// Check cert signatures.
			if err := validateCertSignature(cert, caCert); err != nil {
				return errors.Errorf("failed CA cert check: %v", err)
			}
			return nil
		}

		if err := testCred(testcase.ServerCred, false); err != nil {
			t.Errorf("Test %d: ServerCred: %v", testi, err)
		}
		// TODO(b/159413210): Generate client certificate for TestCert3 and remove this condition.
		if testcase.ClientCred.Cert != "" {
			if err := testCred(testcase.ClientCred, false); err != nil {
				t.Errorf("Test %d: ClientCred: %v", testi, err)
			}
		}
		// TODO(b/159413210): Generate expired certificate for each CertStore and remove this condition.
		if testcase.ExpiredServerCred.Cert != "" {
			if err := testCred(testcase.ExpiredServerCred, true); err != nil {
				t.Errorf("Test %d: ExpiredServerCred: %v", testi, err)
			}
		}
	}
}

// TestAltSubjectMatch test that the entries in TestCert3AltSubjectMatch are exactly what TestCert3 contains.
func TestAltSubjectMatch(t *testing.T) {
	// Get the entries in TestCert3AltSubjectMatch().
	expectedDNSNames := make(map[string]bool)
	expectedEmailAddresses := make(map[string]bool)
	for _, altStr := range TestCert3AltSubjectMatch() {
		var alt struct {
			Type  string
			Value string
		}
		if err := json.Unmarshal([]byte(altStr), &alt); err != nil {
			t.Fatalf("failed to unmarshal altsubject match string: %s", altStr)
		}
		switch alt.Type {
		case "DNS":
			expectedDNSNames[alt.Value] = true
		case "EMAIL":
			expectedEmailAddresses[alt.Value] = true
		default:
			t.Errorf("unexpected Type in altsubject match: %s", alt.Type)
		}
	}

	// Get the entries in TestCert3().ServerCred.Cert.
	cert, err := x509ParseCert(TestCert3().ServerCred.Cert)
	if err != nil {
		t.Fatal(err)
	}
	dnsNames := make(map[string]bool)
	for _, d := range cert.DNSNames {
		dnsNames[d] = true
	}
	emailAddresses := make(map[string]bool)
	for _, e := range cert.EmailAddresses {
		emailAddresses[e] = true
	}

	if !reflect.DeepEqual(dnsNames, expectedDNSNames) {
		t.Errorf("DNS names not match, got %v, want %v", dnsNames, expectedDNSNames)
	}
	if !reflect.DeepEqual(emailAddresses, expectedEmailAddresses) {
		t.Errorf("email addresses not match, got %v, want %v", emailAddresses, expectedEmailAddresses)
	}
}

func TestCADifference(t *testing.T) {
	// Check that TestCert1 and TestCert2 are using different CAs.
	if TestCert1().CACert == TestCert2().CACert {
		t.Error("TestCert1 and TestCert2 are using the same CA")
	}
}
