// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package certificate

import (
	"crypto/x509"
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

func TestCADifference(t *testing.T) {
	// Check that GetTestCertificate and GetTestExpiredCertificate are using the same CA.
	if GetTestCertificate().CACert != GetTestExpiredCertificate().CACert {
		t.Error("GetTestCertificate and GetTestExpiredCertificate are using different CAs")
	}

	// Check that GetTestCertificate and GetTestCertificate2 are using different CAs.
	if GetTestCertificate().CACert == GetTestCertificate2().CACert {
		t.Error("GetTestCertificate and GetTestCertificate2 are using the same CA")
	}
}

func TestCertificate(t *testing.T) {
	now := time.Now()
	isExpired := func(cert *x509.Certificate) bool {
		return now.Before(cert.NotBefore) || now.After(cert.NotAfter)
	}

	// The client cert/key fields are omitted since we have no test using them so far.
	// Copying server cert/key (which is also expired) to them to prevent testing an empty string.
	expiredCertificate := GetTestExpiredCertificate()
	if expiredCertificate.ClientCert != "" {
		t.Fatal("ClientCert of GetExpiredCertificate is not empty")
	}
	expiredCertificate.ClientCert = expiredCertificate.Cert
	if expiredCertificate.ClientPrivateKey != "" {
		t.Fatal("ClientPrivateKey of GetExpiredCertificate is not empty")
	}
	expiredCertificate.ClientPrivateKey = expiredCertificate.PrivateKey

	for testi, testcase := range []struct {
		cert    Certificate
		expired bool
	}{
		{GetTestCertificate(), false},
		{expiredCertificate, true},
		{GetTestCertificate2(), false},
	} {
		c := testcase.cert

		// Parse certificates. They should all be X-509 certificates in PEM format.
		var cert, caCert, clientCert *x509.Certificate
		for i, it := range []struct {
			pem string
			out **x509.Certificate
		}{
			{c.Cert, &cert},
			{c.CACert, &caCert},
			{c.ClientCert, &clientCert},
		} {
			pem, err := pemDecode(it.pem)
			if err != nil {
				t.Fatalf("Test %d: failed to decode PEM %d: %v", testi, i, err)
			}
			*it.out, err = x509.ParseCertificate(pem)
			if err != nil {
				t.Fatalf("Test %d: failed to parse certificate %d: %v", testi, i, err)
			}
		}

		// Verify expiry.
		if expired := isExpired(cert); expired != testcase.expired {
			t.Errorf("Test %d: failed cert expiry check: got %t, want %t", testi, expired, testcase.expired)
		}
		if expired := isExpired(clientCert); expired != testcase.expired {
			t.Errorf("Test %d: failed client cert expiry check: got %t, want %t", testi, expired, testcase.expired)
		}

		// Validate private keys.
		if err := validatePrivateKey(c.PrivateKey, cert); err != nil {
			t.Fatalf("Test %d: failed private key check: %v", testi, err)
		}
		if err := validatePrivateKey(c.ClientPrivateKey, clientCert); err != nil {
			t.Fatalf("Test %d: failed client private key check: %v", testi, err)
		}

		// Check cert signatures.
		if err := validateCertSignature(cert, caCert); err != nil {
			t.Errorf("Test %d: failed CA cert check: %v", testi, err)
		}
		// Check clientCert signatures.
		if err := validateCertSignature(clientCert, caCert); err != nil {
			t.Errorf("Test %d: failed client CA cert check: %v", testi, err)
		}

		if err := validateCertSignature(caCert, caCert); err != nil {
			t.Errorf("Test %d: unexpeted: CA cert isn't self-signed", testi)
		}
	}
}
