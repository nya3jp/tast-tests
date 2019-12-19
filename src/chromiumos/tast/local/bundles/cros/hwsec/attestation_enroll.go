// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AttestationEnroll,
		Desc: "Verifies pre-condition and post-contidion of enrollment",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome"},
	})
}

// AttestationEnroll Verifies pre-condition and post-contidion of enrollment.
func AttestationEnroll(ctx context.Context, s *testing.State) {
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
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")

	s.Log("Checking if enrollment can be done w/o owner password")
	// Logs in if there might be no first login happens before.
	if passwd, err := utility.GetOwnerPassword(ctx); err != nil || len(passwd) != 0 {
		auth := chrome.Auth("test@crashwsec.bigr.name", "testpass", "gaia-id")
		cr, err := chrome.New(ctx, auth)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		cr.Close(ctx)
		s.Log("Restarting attestation/cryptohome so we can ensure no cached owner password")
		dCtrl := hwsec.NewDaemonController(r)
		dCtrl.RestartAttestation(ctx)
		dCtrl.RestartCryptohome(ctx)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Presumably, all owner dependency should be automatically triggered after TPM ownerhsip is taken.
			// Thus, just poll the owner password.
			if s, err := utility.GetOwnerPassword(ctx); err != nil {
				return err
			} else if len(s) != 0 {
				return errors.New("Non-empty password")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			s.Fatal("Failed to wait for owner password to be cleared: ", err)
		}
	}

	req, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	s.Log(len(req))
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	resp, err := attestation.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err := utility.IsEnrolled(ctx)
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	s.Log("Enrolling DUT with the same response")
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}

	// auth := chrome.Auth("cros@crosdmsregtest.com", "testpass", "gaia-id")
	auth := chrome.Auth("test@crashwsec.bigr.name", "testpass", "gaia-id")
	cr, err := chrome.New(ctx, auth)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)
	username := cr.User()
	s.Log("Chrome user: ", username)

	req, err = utility.CreateCertRequest(
		ctx,
		hwsec.DefaultPCA,
		attestation.DefaultCertProfile,
		username,
		attestation.DefaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")
	s.Log("Sending sign request")
	resp, err = attestation.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = attestation.SendPostRequestTo(req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Error("Unexpected empty cert")
		return
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(ctx, resp, username, attestation.DefaultCertLabel)

	if err != nil {
		s.Error("Failed to finish cert request: ", err)
		return
	}
	s.Log("Finished certificate request")

	s.Log("Checking if the cert is still valid after enrollment again")
	// Presumably the signed challenge here is correct because it's validates
	// in the basic test flow "Attesation".
	// Thus, skips the closed loop test here.
	signedChallenge, err := utility.SignSimpleChallenge(
		ctx,
		username,
		attestation.DefaultCertLabel,
		[]byte{})
	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	publicKeyHex, err := utility.GetPublicKey(ctx, username, attestation.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}

	s.Log("Enrollment again")
	req, err = utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	s.Log(len(req))
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	resp, err = attestation.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err = utility.IsEnrolled(ctx)
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	signedChallenge2, err := utility.SignSimpleChallenge(
		ctx,
		username,
		attestation.DefaultCertLabel,
		[]byte{})
	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	publicKeyHex2, err := utility.GetPublicKey(ctx, username, attestation.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}
	if publicKeyHex != publicKeyHex2 {
		s.Fatal("Public key differed")
	}
	publicKeyDer, err := attestation.HexDecode([]byte(publicKeyHex))
	if err != nil {
		s.Fatal("hex-decode public key: ", err)
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDer)
	if err != nil {
		s.Fatal("Failed to construct rsa public key")
	}
	for _, sc := range []string{signedChallenge, signedChallenge2} {
		signedChallengeProto, err := attestation.UnmarshalSignedData([]byte(sc))
		hashValue := sha256.Sum256(signedChallengeProto.GetData())

		err = rsa.VerifyPKCS1v15(
			publicKey.(*rsa.PublicKey),
			crypto.SHA256,
			hashValue[:],
			signedChallengeProto.GetSignature())
		if err != nil {
			s.Fatal("Failed to verify signature...what~~~")
		}

		s.Log("Finished Verifying simple challenge")

	}

}
