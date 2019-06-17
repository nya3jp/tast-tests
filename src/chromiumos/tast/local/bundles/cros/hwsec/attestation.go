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
	"encoding/base64"
	"net/url"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Attestation,
		Desc: "Verifies attestation-related functionality",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome", "tpm"},
	})
}

// Attestation runs through the attestation flow, including enrollment, cert, sign challenge.
// Also, it verifies the the key access functionality.
func Attestation(ctx context.Context, s *testing.State) {
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
	req, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	//resp, err := attestation.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Fatal("Failed to send request to CA: ", err)
	}
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Fatal("Failed to finish enrollment: ", err)
	}
	isEnrolled, err := utility.IsEnrolled(ctx)
	if err != nil {
		s.Fatal("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Fatal("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	s.Log("Creating ceritificate request")

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
		s.Fatal("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Sending sign request")
	resp, err = attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
	if err != nil {
		s.Fatal("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Fatal("Unexpected empty cert")
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(ctx, resp, username, attestation.DefaultCertLabel)

	if err != nil {
		s.Fatal("Failed to finish cert request: ", err)
	}
	s.Log("Finished certificate request")

	s.Log("Verifying VA challenge funcationality")

	s.Log("Getting challenge from VA server")
	resp, err = attestation.SendGetRequestTo(ctx, "https://test-dvproxy-server.sandbox.google.com/dvproxy/getchallenge")
	if err != nil {
		s.Fatal("Failed to send request to VA: ", err)
	}
	challenge, err := base64.StdEncoding.DecodeString(resp)
	if err != nil {
		s.Fatal("Failed to base64-decode challenge: ", err)
	}
	s.Log("Singing the challenge")
	signedChallenge, err := utility.SignEnterpriseVAChallenge(
		ctx,
		0,
		username,
		attestation.DefaultCertLabel,
		username,
		"fake_device_id",
		true,
		challenge)
	b64SignedChallenge := base64.StdEncoding.EncodeToString([]byte(signedChallenge))
	if err != nil {
		s.Fatal("Failed to sign VA challenge: ", err)
	} else {
		s.Log(b64SignedChallenge)
	}
	s.Log("Challenge signed")
	s.Log("Sending the singed challenge back for verification")
	urlForVerification := "https://test-dvproxy-server.sandbox.google.com/dvproxy/verifychallengeresponse?signeddata=" + url.QueryEscape(b64SignedChallenge)
	s.Log(urlForVerification)
	resp, err = attestation.SendGetRequestTo(ctx, urlForVerification)
	if err != nil {
		s.Fatal("Failed to verify challenge: ", err)
	}
	s.Log("Challenge verified. response: ", resp)

	s.Log("Verifying simple challenge functionality")

	signedChallenge, err = utility.SignSimpleChallenge(
		ctx,
		username,
		attestation.DefaultCertLabel,
		[]byte{})

	if err != nil {
		s.Fatal("Failed to sign simple challenge")
	}
	signedChallengeProto, err := attestation.UnmarshalSignedData([]byte(signedChallenge))
	if err != nil {
		s.Fatal("Failed to unmarshal simple challenge reply")
	}
	s.Log(signedChallengeProto)
	s.Log("Verifying signature")

	publicKeyHex, err := utility.GetPublicKey(ctx, username, attestation.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}
	publicKeyDer, err := attestation.HexDecode([]byte(publicKeyHex))
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
		s.Fatal("Failed to verify signature")
	}

	s.Log("Finished Verifying simple challenge")

	s.Log("Start key payload closed-loop testing")
	s.Log("Setting key payload")
	expectedPayload := attestation.DefaultKeyPayload
	_, err = utility.SetKeyPayload(ctx, username, attestation.DefaultCertLabel, expectedPayload)
	if err != nil {
		s.Fatal("Failed to set key payload: ", err)
	}
	s.Log("Getting key payload")
	resultPayload, err := utility.GetKeyPayload(ctx, username, attestation.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get key payload: ", err)
	}
	if resultPayload != expectedPayload {
		s.Fatalf("Inconsistent paylaod -- result: %s / expected: %s", resultPayload, expectedPayload)
	}
	s.Log("Start key payload closed-loop done")

	s.Log("Start verifying key registration")
	isSuccessful, err := utility.RegisterKeyWithChapsToken(ctx, username, attestation.DefaultCertLabel)
	if err != nil {
		s.Fatal("Failed to register key with chaps token due to error: ", err)
	}
	if !isSuccessful {
		s.Fatal("Failed to register key with chaps token")
	}
	// Now the key has been registered and remove from the key store
	_, err = utility.GetPublicKey(ctx, username, attestation.DefaultCertLabel)
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
			hwsec.DefaultPCA,
			attestation.DefaultCertProfile,
			username,
			attestation.DefaultCertOrigin)
		if err != nil {
			s.Fatal("Failed to create certificate request: ", err)
		}
		s.Log("Created certificate request")

		s.Log("Sending sign request")
		resp, err = attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
		if err != nil {
			s.Fatal("Failed to send request to CA: ", err)
		}
		if len(resp) == 0 {
			s.Fatal("Unexpected empty cert")
		}
		err = utility.FinishCertRequest(ctx, resp, username, label)
		if err != nil {
			s.Fatal("Failed to finish cert request: ", err)
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
