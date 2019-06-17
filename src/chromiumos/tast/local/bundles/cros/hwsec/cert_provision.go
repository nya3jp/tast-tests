// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"strings"

	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CertProvision,
		Desc: "Verifies cert provision by closed loop testing",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
	})
}

func CertProvision(ctx context.Context, s *testing.State) {
	helper, err := libhwseclocal.NewHelperLocal()
	if err != nil {
		s.Fatal("Error creating helper")
	}
	const (
		execName          = "cert_provision_client"
		provisionCmd      = "--provision"
		getCmd            = "--get"
		signCmd           = "--sign"
		defaultPCAOpt     = "--pca=default"
		defaultLabelOpt   = "--label=cp9487"
		defaultProfileopt = "--profile=jetstream"
		defaultDataFile   = "/tmp/cpdata5487"
	)

	out, err := helper.Run(
		ctx,
		execName,
		provisionCmd,
		defaultProfileopt,
		defaultPCAOpt,
		defaultLabelOpt)
	if err != nil {
		s.Fatal("Failed to provision: ", string(out))
	}
	out, err = helper.Run(
		ctx,
		execName,
		getCmd,
		defaultLabelOpt,
		"--include_chain")
	if err != nil {
		s.Fatal("Failed to get registered key: ", string(out))
	}
	s.Log(string(out))

	block, _ := pem.Decode(out)
	if block == nil {
		s.Fatal("Failed to decode returned PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		s.Fatal("Failed to parse cert: ", err)
	}
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	s.Log(rsaPublicKey)

	for _, mechanism := range []string{"sha1_rsa", "sha256_rsa"} {
		_, err := helper.Run(
			ctx,
			"dd",
			"if=/dev/urandom",
			"bs=10",
			"count=1",
			"of="+defaultDataFile)
		if err != nil {
			s.Fatal("Failed to generate data")
		}
		data, err := helper.Run(ctx, "cat", defaultDataFile)
		if len(data) != 10 {
			s.Fatal("Unexpected length of the data read (should be 10): ", len(data))
		}
		out, err := helper.Run(
			ctx,
			execName,
			signCmd,
			defaultLabelOpt,
			"--in="+defaultDataFile,
			"--mechanism="+mechanism)
		if err != nil {
			s.Fatal("Failed to sign: ", string(out))
		}
		hashType, hashValue := func() (crypto.Hash, []byte) {
			if mechanism == "sha1_rsa" {
				h := sha1.Sum(data)
				return crypto.SHA1, h[:]
			}
			h := sha256.Sum256(data)
			return crypto.SHA256, h[:]
		}()
		trimmedOut := []byte(strings.TrimSpace(string(out)))
		signature, err := hexDecode(trimmedOut)
		if err != nil {
			s.Fatal("Failed to hex-decode the signature: ", err)
		}
		err = rsa.VerifyPKCS1v15(
			rsaPublicKey,
			hashType,
			hashValue,
			signature)
		if err != nil {
			s.Fatal("failed to verify signature...what~~~")
		}
	}

}
