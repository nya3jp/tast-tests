// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
)

// Key generating wrappers.

// KeyType is the type of key and also a key generating function.
type KeyType func(io.Reader) (crypto.PrivateKey, crypto.PublicKey, error)

// RSAKey returns a KeyType of RSA with 1024 bits key length.
func RSAKey() KeyType {
	return RSAKeyWithBits(1024)
}

// RSAKeyWithBits returns a KeyType of RSA with given key length in bits.
func RSAKeyWithBits(bits int) KeyType {
	return func(r io.Reader) (crypto.PrivateKey, crypto.PublicKey, error) {
		k, err := rsa.GenerateKey(r, bits)
		if err != nil {
			return nil, nil, err
		}
		return k, &k.PublicKey, nil
	}
}

// ECDSAKeyWithCurve return a KeyType of ECDSA with the given curve.
func ECDSAKeyWithCurve(c elliptic.Curve) KeyType {
	return func(r io.Reader) (crypto.PrivateKey, crypto.PublicKey, error) {
		k, err := ecdsa.GenerateKey(c, r)
		if err != nil {
			return nil, nil, err
		}
		return k, &k.PublicKey, nil
	}
}

// Formatter provides the interface of objects formating private keys and
// certificates.
// FormatKey takes a PrivateKey object directly.
// FormatCert takes the certificate in DER.
type Formatter interface {
	FormatKey(crypto.PrivateKey) (string, error)
	FormatCert([]byte) (string, error)
}

// defaultFormater formats keys in PKCS1 PEM format and certs in PEM format.
type defaultFormater struct{}

func (f defaultFormater) pemEncode(t string, content []byte) (string, error) {
	block := &pem.Block{
		Type:  t,
		Bytes: content,
	}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, block); err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}

func (f defaultFormater) FormatKey(key crypto.PrivateKey) (string, error) {
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("default formater only support RSA key")
	}
	der := x509.MarshalPKCS1PrivateKey(rsaKey)
	return f.pemEncode("RSA PRIVATE KEY", der)
}

func (f defaultFormater) FormatCert(der []byte) (string, error) {
	return f.pemEncode("CERTIFICATE", der)
}

var serialNumber int64

// getSerial is a utilities for generating deterministic but unique serial numbers
// for certifactes.
func getSerial() *big.Int {
	ret := big.NewInt(serialNumber)
	serialNumber++
	return ret
}
