// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CertProvisionNoDatabase,
		Desc:         "Verifies cert provision is still working without attestation database",
		Attr:         []string{"group:mainline", "disabled"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm"},
	})
}

func CertProvisionNoDatabase(ctx context.Context, s *testing.State) {
	r, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}
	s.Log("Attestation is prepared for enrollment")
	const (
		execName          = "cert_provision_client"
		provisionCmd      = "--provision"
		getCmd            = "--get"
		signCmd           = "--sign"
		defaultPCAOpt     = "--pca=default"
		defaultLabelOpt   = "--label=cp9487"
		defaultProfileOpt = "--profile=jetstream"
		defaultDataFile   = "/tmp/cpdata5487"
	)

	out, err := r.Run(
		ctx,
		execName,
		provisionCmd,
		defaultProfileOpt,
		defaultPCAOpt,
		defaultLabelOpt)
	if err != nil {
		s.Fatal("Failed to provision: ", err)
	}
	s.Log("Emulating Cleared attetation database")
	fw := hwsec.NewFileWiper(r)
	dCtrl := hwsec.NewDaemonController(r)

	if err := func() (retErr error) {
		if err := dCtrl.StopAttestation(ctx); err != nil {
			return errors.Wrap(err, "failed to stop attestationd")
		}
		defer func() {
			if err := dCtrl.StartAttestation(ctx); err != nil && retErr == nil {
				retErr = errors.Wrap(err, "failed to restart attestationd")
			}
		}()
		return fw.Wipe(ctx, hwsec.AttestationDBPath)
	}(); err != nil {
		s.Fatal("Failed to wipe out attestation database: ", err)
	}

	defer func() {
		s.Log("Restoring attestation DB")
		if err := dCtrl.StopAttestation(ctx); err != nil {
			s.Error("Failed to stop attestationd: ", err)
		} else {
			defer func() {
				if err := dCtrl.StartAttestation(ctx); err != nil {
					s.Error("Failed to start attestationd: ", err)
				}
			}()
		}
		// Still tried to restore and the database even if the daemon is not stopped successfully.
		if err := fw.Restore(ctx, hwsec.AttestationDBPath); err != nil {
			s.Error("Failed to restore attestation database: ", err)
		}
	}()

	out, err = r.Run(
		ctx,
		execName,
		getCmd,
		defaultLabelOpt,
		"--include_chain")
	if err != nil {
		s.Fatal("Failed to get registered key: ", err)
	}

	block, _ := pem.Decode(out)
	if block == nil {
		s.Fatal("Failed to decode returned PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		s.Fatal("Failed to parse cert: ", err)
	}
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)

	for _, mechanism := range []string{"sha1_rsa", "sha256_rsa"} {
		s.Run(ctx, mechanism, func(ctx context.Context, s *testing.State) {
			_, err := r.Run(ctx, "dd", "if=/dev/urandom", "bs=10", "count=1", "of="+defaultDataFile)
			if err != nil {
				s.Fatal("Failed to generate data: ", err)
			}
			data, err := r.Run(ctx, "cat", defaultDataFile)
			if len(data) != 10 {
				s.Fatal("Unexpected length of the data read (should be 10): ", len(data))
			}
			out, err := r.Run(ctx, execName, signCmd, defaultLabelOpt, "--in="+defaultDataFile, "--mechanism="+mechanism)
			if err != nil {
				s.Fatalf("Failed to sign %v: %v", string(out), err)
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
			signature, err := hwsec.HexDecode(trimmedOut)
			if err != nil {
				s.Fatal("Failed to hex-decode the signature: ", err)
			}
			err = rsa.VerifyPKCS1v15(rsaPublicKey, hashType, hashValue, signature)
			if err != nil {
				s.Fatal("Failed to verify signature: ", err)
			}
		})
	}
}
