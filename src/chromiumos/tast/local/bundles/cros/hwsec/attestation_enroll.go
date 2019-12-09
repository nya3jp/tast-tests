// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	libhwseclocal "chromiumos/tast/local/hwsec"
	a9n "chromiumos/tast/local/hwsec/attestation"
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
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	helper, err := libhwseclocal.NewHelper()
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, r, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, utility, a9n.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx,
		utility, a9n.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")

	s.Log("Checking if enrollment can be done w/o owner password")
	// Logs in if there might be no first login happens before.
	if passwd, err := utility.GetOwnerPassword(); err != nil || len(passwd) != 0 {
		auth := chrome.Auth("test@crashwsec.bigr.name", "testpass", "gaia-id")
		cr, err := chrome.New(ctx, auth)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		cr.Close(ctx)
		s.Log("Restarting attestation/cryptohome so we can ensure no cached owner password")
		libhwsec.RestartAttestation(ctx, r)
		libhwsec.RestartCryptohome(ctx, r)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Presumably, all owner dependency should be automatically triggered after TPM ownerhsip is taken.
			// Thus, just poll the owner password.
			if s, err := utility.GetOwnerPassword(); err != nil {
				return err
			} else if len(s) != 0 {
				return errors.New("Non-empty password")
			}
			return nil
		}, &testing.PollOptions{Interval: 1000 * time.Millisecond, Timeout: time.Minute}); err != nil {
			s.Fatal("Failed to wait for owner password to be cleared: ", err)
		}
	}

	req, err := utility.CreateEnrollRequest(a9n.DefaultACAType)
	s.Log(len(req))
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	resp, err := a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	err = utility.FinishEnroll(a9n.DefaultACAType, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err := utility.IsEnrolled()
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	s.Log("Enrolling DUT with the same response")
	err = utility.FinishEnroll(a9n.DefaultACAType, resp)
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
		a9n.DefaultACAType,
		a9n.DefaultCertProfile,
		username,
		a9n.DefaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")
	s.Log("Sending sign request")
	resp, err = a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = a9n.SendPostRequestTo(req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Error("Unexpected empty cert")
		return
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(resp, username, a9n.DefaultCertLabel)

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
		username,
		a9n.DefaultCertLabel,
		[]byte{})
	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	publicKeyHex, err := utility.GetPublicKey(username, a9n.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}

	s.Log("Enrollment again")
	req, err = utility.CreateEnrollRequest(a9n.DefaultACAType)
	s.Log(len(req))
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	resp, err = a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	err = utility.FinishEnroll(a9n.DefaultACAType, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err = utility.IsEnrolled()
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	signedChallenge2, err := utility.SignSimpleChallenge(
		username,
		a9n.DefaultCertLabel,
		[]byte{})
	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	publicKeyHex2, err := utility.GetPublicKey(username, a9n.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}
	if publicKeyHex != publicKeyHex2 {
		s.Fatal("Public key differed")
	}
	publicKeyDer, err := a9n.HexDecode([]byte(publicKeyHex))
	if err != nil {
		s.Fatal("hex-decode public key: ", err)
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDer)
	if err != nil {
		s.Fatal("Failed to construct rsa public key")
	}
	for _, sc := range []string{signedChallenge, signedChallenge2} {
		signedChallengeProto, err := a9n.UnmarshalSignedData([]byte(sc))
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
