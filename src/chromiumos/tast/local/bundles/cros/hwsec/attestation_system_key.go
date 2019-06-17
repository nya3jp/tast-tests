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

	"chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	a9n "chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AttestationSystemKey,
		Desc: "Verifies certified system key",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func AttestationSystemKey(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := libhwseclocal.NewHelper(utility)
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
	// For now, just uses '0' as in "default CA"
	req, err := utility.CreateEnrollRequest(ctx, a9n.DefaultACAType)
	s.Log(len(req))
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
		return
	}
	//resp, err := a9n.SendPostRequestTo(req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := a9n.SendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	// For now, just uses '0' as in "default CA"
	err = utility.FinishEnroll(ctx, a9n.DefaultACAType, resp)
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

	s.Log("Creating ceritificate request")

	// Empty username indicates the system-level key.
	username := ""

	req, err = utility.CreateCertRequest(
		ctx,
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
	err = utility.FinishCertRequest(ctx, resp, username, a9n.DefaultCertLabel)

	if err != nil {
		s.Error("Failed to finish cert request: ", err)
		return
	}
	s.Log("Finished certificate request")
	s.Log("Verifying simple challenge functionality")

	signedChallenge, err := utility.SignSimpleChallenge(
		ctx,
		username,
		a9n.DefaultCertLabel,
		[]byte{})

	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	signedChallengeProto, err := a9n.UnmarshalSignedData([]byte(signedChallenge))
	if err != nil {
		s.Fatal("Failed to unmarshal simple challenge reply")
	}
	s.Log(signedChallengeProto)
	s.Log("Verifying signature")

	publicKeyHex, err := utility.GetPublicKey(ctx, username, a9n.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}
	publicKeyDer, err := a9n.HexDecode([]byte(publicKeyHex))
	if err != nil {
		s.Fatal("hex-decode public key: ", err)
	}
	s.Log(publicKeyDer)
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDer)
	if err != nil {
		s.Fatal("Failed to construct rsa public key")
	}
	s.Log(publicKey)
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

	s.Log("Start key payload closed-loop testing")
	s.Log("Setting key payload")
	expectedPayload := a9n.DefaultKeyPayload
	_, err = utility.SetKeyPayload(ctx, username, a9n.DefaultCertLabel, expectedPayload)
	if err != nil {
		s.Fatal("Failed to set key payload: ", err)
	}
	s.Log("Getting key payload")
	resultPayload, err := utility.GetKeyPayload(ctx, username, a9n.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get key payload: ", err)
	}
	if resultPayload != expectedPayload {
		s.Fatalf("Inconsistent paylaod -- result: %s / expected: %s", resultPayload, expectedPayload)
	}
	s.Log("Start key payload closed-loop done")

	s.Log("Start verifying key registration")
	isSuccessful, err := utility.RegisterKeyWithChapsToken(ctx, username, a9n.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to register key with chaps token due to error: ", err)
	}
	if !isSuccessful {
		s.Fatal("Failed to register key with chaps token")
	}
	// Now the key has been registered and remove from the key store
	_, err = utility.GetPublicKey(ctx, username, a9n.DefaultCertLabel)
	if err == nil {
		s.Fatal("unsidered successful operation -- key should be removed after registration")
	}
	// Well, actually we need more on system key so the key registration is validated.
	s.Log("Key registration verified")

	s.Log("Verifying deletion of keys by prefix")
	for _, label := range []string{"label1", "label2", "label3"} {
		s.Log("cert process for label: ", label)
		req, err = utility.CreateCertRequest(
			ctx,
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
		err = utility.FinishCertRequest(ctx, resp, username, label)
		if err != nil {
			s.Error("Failed to finish cert request: ", err)
			return
		}
		_, err := utility.GetPublicKey(ctx, username, label)
		if err != nil {
			s.Fatal("Failed to get public key: ", err)
		}
	}
	s.Log("Deleting keys just created")
	if err := utility.DeleteKeys(ctx, username, "label"); err != nil {
		s.Fatal("Failed to remove the key group: ", err)
	}
	for _, label := range []string{"label1", "label2", "label3"} {
		s.Log("Checking if key is deleted...label=", label)
		if _, err := utility.GetPublicKey(ctx, username, label); err == nil {
			s.Fatal("Failed to get public key: ", err)
		}
	}
	s.Log("Deletion of keys by prefix verified")
}
