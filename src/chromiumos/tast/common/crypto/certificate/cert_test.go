// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"reflect"
	"testing"

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

func TestCertificates(t *testing.T) {
	c := TestCertificate()

	// Parse certificates. They should all be X-509 certificates in PEM format.
	var cert, caCert, clientCert *x509.Certificate
	for i, it := range []struct {
		pem string
		out **x509.Certificate
	}{
		{
			c.Cert, &cert,
		},
		{
			c.CACert, &caCert,
		},
		{
			c.ClientCert, &clientCert,
		},
	} {
		pem, err := pemDecode(it.pem)
		if err != nil {
			t.Fatalf("Failed to decode PEM %d: %v", i, err)
		}
		*it.out, err = x509.ParseCertificate(pem)
		if err != nil {
			t.Fatalf("Failed to parse certificate %d: %v", i, err)
		}
	}

	// Validate private keys.
	if err := validatePrivateKey(c.PrivateKey, cert); err != nil {
		t.Fatal("Failed private key check: ", err)
	}
	if err := validatePrivateKey(c.ClientPrivateKey, clientCert); err != nil {
		t.Fatal("Failed client private key check: ", err)
	}

	// Check cert signatures.
	if err := validateCertSignature(cert, caCert); err != nil {
		t.Error("Failed CA cert check: ", err)
	}
	// Check clientCert signatures.
	if err := validateCertSignature(clientCert, caCert); err != nil {
		t.Error("Failed client CA cert check: ", err)
	}

	if err := validateCertSignature(caCert, caCert); err != nil {
		t.Error("Unexpeted: CA cert isn't self-signed")
	}
}
