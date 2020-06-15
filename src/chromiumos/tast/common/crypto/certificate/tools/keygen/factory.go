// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"time"

	"chromiumos/tast/common/crypto/certificate"
)

// Option is the function signature used to specify options of Factory.
type Option func(*Factory)

// Factory generates a set of keys and certificates for Certificate
// object so that tests can use them.
type Factory struct {
	caSigAlgo x509.SignatureAlgorithm
	sigAlgo   x509.SignatureAlgorithm
	keyGen    KeyType
	formatter Formatter
}

// NewFactory create a Factory with given options.
func NewFactory(ops ...Option) *Factory {
	f := &Factory{
		caSigAlgo: x509.SHA1WithRSA,
		sigAlgo:   x509.MD5WithRSA,
		keyGen:    RSAKey(),
		formatter: defaultFormater{},
	}
	for _, op := range ops {
		op(f)
	}
	return f
}

// subject returns the subject for the certificate with given name.
func (f *Factory) subject(name string) pkix.Name {
	return pkix.Name{
		Country:    []string{"US"},
		Locality:   []string{"Mountain View"},
		Province:   []string{"California"},
		CommonName: fmt.Sprintf("chromelab-wifi-testbed-%s.mtv.google.com", name),
	}
}

// genCertOption is the internal option for Factory to generate
// keys and certs with different options.
type genCertOption func(*genCertConfig)

// notBefore sets the NotBefore field of certificate template.
func notBefore(t time.Time) genCertOption {
	return func(c *genCertConfig) {
		c.template.NotBefore = t
	}
}

// notAfter sets the NotAfter field of certificate template.
func notAfter(t time.Time) genCertOption {
	return func(c *genCertConfig) {
		c.template.NotAfter = t
	}
}

// signer sets the key and certificate of signer for the generated
// certificate.
func signer(key crypto.PrivateKey, parent *x509.Certificate) genCertOption {
	return func(c *genCertConfig) {
		c.signKey = key
		c.parent = parent
	}
}

// isCA sets the IsCA field of certificate template to true.
func isCA() genCertOption {
	return func(c *genCertConfig) {
		c.template.IsCA = true
	}
}

// sigAlgo sets the SignatureAlgorithm field of certificate template.
func sigAlgo(algo x509.SignatureAlgorithm) genCertOption {
	return func(c *genCertConfig) {
		c.template.SignatureAlgorithm = algo
	}
}

// genCertConfig is the internal structure for holding options passed
// to Factory.genCert call.
type genCertConfig struct {
	template x509.Certificate

	signKey crypto.PrivateKey
	parent  *x509.Certificate
}

// genCert generates a pair of key and certificate of the public key with the signer
// specified. (if no signer option is passed, it will be self-signed)
func (f *Factory) genCert(rand io.Reader, name string, ops ...genCertOption) (privKey crypto.PrivateKey, cert *x509.Certificate, keyStr, certStr string, retErr error) {
	// TODO: still many fields to fill.
	config := &genCertConfig{
		template: x509.Certificate{
			SignatureAlgorithm: x509.SHA1WithRSA,
			IsCA:               false,
			NotBefore:          time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			NotAfter:           time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			Subject:            f.subject(name),
			SerialNumber:       getSerial(),
		},
	}
	for _, op := range ops {
		op(config)
	}

	key, pubKey, err := f.keyGen(rand)
	if err != nil {
		return nil, nil, "", "", err
	}
	// Self-sign if no signer.
	if config.signKey == nil {
		config.signKey = key
		config.parent = &config.template
	}

	der, err := x509.CreateCertificate(rand, &config.template, config.parent, pubKey, config.signKey)
	if err != nil {
		return nil, nil, "", "", err
	}
	// Make it back to a *Certificate so that it might be used as parent.
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, "", "", err
	}
	// Format key and cert into string.
	formatKey, err := f.formatter.FormatKey(key)
	if err != nil {
		return nil, nil, "", "", err
	}
	formatCert, err := f.formatter.FormatCert(der)
	if err != nil {
		return nil, nil, "", "", err
	}
	return key, cert, formatKey, formatCert, nil
}

// Gen generates a certificate.Certifate with given random source.
func (f *Factory) Gen(rand io.Reader) (*certificate.Certificate, error) {
	cert := &certificate.Certificate{}

	// Generate root key and self-signed cert.
	caKey, caCert, _, caCertStr, err := f.genCert(rand, "root", isCA(), sigAlgo(f.caSigAlgo))
	if err != nil {
		return nil, err
	}
	cert.CACert = caCertStr

	// Generate server key and cert.
	_, _, cert.PrivateKey, cert.Cert, err = f.genCert(rand, "server", signer(caKey, caCert), sigAlgo(f.sigAlgo))
	if err != nil {
		return nil, err
	}
	// Generate client key and cert.
	_, _, cert.ClientPrivateKey, cert.ClientCert, err = f.genCert(rand, "client", signer(caKey, caCert), sigAlgo(f.sigAlgo))
	if err != nil {
		return nil, err
	}

	return cert, nil
}
