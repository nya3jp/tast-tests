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

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
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
		SoftwareDeps: []string{"chrome"},
	})
}

// Attestation runs through the attestation flow, including enrollment, cert, sign challenge.
// Also, it verifies the the key access functionality.
func Attestation(ctx context.Context, s *testing.State) {
	s.Log("Start test with creating a proxy")
	utility, err := libhwsec.NewUtility(ctx, s, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Error("Utilty creation error: ", err)
		return
	}
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, defaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("Tpm is ensured to be ready")
	if err := libhwsec.EnsureIsPreparedForEnrollment(ctx,
		utility, defaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")
	// For now, just uses '0' as in "default CA"
	req, err := utility.CreateEnrollRequest(defaultACAType)
	s.Log(len(req))
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
		return
	}
	//resp, err := sendPostRequestTo(req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := sendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	// For now, just uses '0' as in "default CA"
	err = utility.FinishEnroll(defaultACAType, resp)
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

	s.Log("Creating ceritificate request")

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
		defaultACAType,
		defaultCertProfile,
		username,
		defaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Sending sign request")
	resp, err = sendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = sendPostRequestTo(req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Error("Unexpected empty cert")
		return
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(resp, username, defaultCertLabel)

	if err != nil {
		s.Error("Failed to finish cert request: ", err)
		return
	}
	s.Log("Finished certificate request")

	s.Log("Verifying VA challenge funcationality")

	s.Log("Getting challenge from VA server")
	resp, err = sendGetRequestTo("https://test-dvproxy-server.sandbox.google.com/dvproxy/getchallenge")
	//resp, err = sendGetRequestTo("https://qa-dvproxy-server-gws.sandbox.google.com/dvproxy/getchallenge")
	if err != nil {
		s.Error("Failed to send request to VA: ", err)
	}
	challenge, err := decodeBase64String(resp)
	if err != nil {
		s.Error("Failed to base64-decode challenge: ", err)
	}
	s.Log("Singing the challenge")
	signedChallenge, err := utility.SignEnterpriseVAChallenge(
		0,
		username,
		defaultCertLabel,
		username,
		"fake_device_id",
		true,
		challenge)
	b64SignedChallenge := encodeToBase64String([]byte(signedChallenge))
	if err != nil {
		s.Error("Failed to sign VA challenge: ", err)
	} else {
		s.Log(b64SignedChallenge)
	}
	s.Log("Challenge signed.")
	s.Log("Sending the singed challenge back for verification...")
	urlForVerification := "https://test-dvproxy-server.sandbox.google.com/dvproxy/verifychallengeresponse?signeddata=" + escapeUrl(b64SignedChallenge)
	s.Log(urlForVerification)
	resp, err = sendGetRequestTo(urlForVerification)
	if err != nil {
		s.Error("Failed to verify challenge: ", err)
	}
	s.Log("Challenge verified. response: ", resp)

	s.Log("Verifying simple challenge functionality")

	signedChallenge, err = utility.SignSimpleChallenge(
		username,
		defaultCertLabel,
		[]byte{})

	if err != nil {
		s.Fatal("Failed to sign simple challenge.")
	}
	signedChallengeProto, err := unmarshalSignedData([]byte(signedChallenge))
	if err != nil {
		s.Fatal("Failed to unmarshal simple challenge reply")
	}
	s.Log(signedChallengeProto)
	s.Log("Verifying signature...")

	publicKeyHex, err := utility.GetPublicKey(username, defaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get public key from service: ", err)
	}
	publicKeyDer, err := hexDecode([]byte(publicKeyHex))
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
		s.Fatal("failed to verify signature...what~~~")
	}

	s.Log("Finished Verifying simple challenge")

	s.Log("Start key payload closed-loop testing...")
	s.Log("Setting key payload")
	expectedPayload := defaultKeyPayload
	_, err = utility.SetKeyPayload(username, defaultCertLabel, expectedPayload)
	if err != nil {
		s.Fatal("Failed to set key payload: ", err)
	}
	s.Log("Getting key payload")
	resultPayload, err := utility.GetKeyPayload(username, defaultCertLabel)
	if err != nil {
		s.Fatal("Failed to get key payload: ", err)
	}
	if resultPayload != expectedPayload {
		s.Fatal("Inconsistent paylaod -- result: %s / expected: %s", resultPayload, expectedPayload)
	}
	s.Log("Start key payload closed-loop done")

	s.Log("Start verifying key registration")
	isSuccessful, err := utility.RegisterKeyWithChapsToken(username, defaultCertLabel)
	if err != nil {
		s.Fatal("failed to register key with chaps token due to error: ", err)
	}
	if !isSuccessful {
		s.Fatal("failed to register key with chaps token")
	}
	// Now the key has been registered and remove from the key store
	_, err = utility.GetPublicKey(username, defaultCertLabel)
	if err == nil {
		s.Fatal("unsidered successful operation -- key should be removed after registration.")
	}
	// Well, actually we need more on system key so the key registration is validated.
	s.Log("Key registration verified")

	s.Log("Verifying deletion of keys by prefix...")
	for _, label := range []string{"label1", "label2", "label3"} {
		s.Log("cert process for label: ", label)
		req, err = utility.CreateCertRequest(
			defaultACAType,
			defaultCertProfile,
			username,
			defaultCertOrigin)
		if err != nil {
			s.Error("Failed to create certificate request: ", err)
		}
		s.Log("Created certificate request")

		s.Log("Sending sign request")
		resp, err = sendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
		//resp, err = sendPostRequestTo(req, "https://asbestos-qa.corp.google.com/sign")
		if err != nil {
			s.Error("Failed to send request to CA: ", err)
		}
		if len(resp) == 0 {
			s.Error("Unexpected empty cert")
			return
		}
		err = utility.FinishCertRequest(resp, username, label)
		if err != nil {
			s.Error("Failed to finish cert request: ", err)
			return
		}
		_, err := utility.GetPublicKey(username, label)
		if err != nil {
			s.Fatal("Failed to get public key: ", err)
		}
	}
	s.Log("Deleting keys just created.")
	if err := utility.DeleteKeys(username, "label"); err != nil {
		s.Fatal("Failed to remove the key group: ", err)
	}
	for _, label := range []string{"label1", "label2", "label3"} {
		s.Log("Checking if key is deleted...label=", label)
		if _, err := utility.GetPublicKey(username, label); err == nil {
			s.Fatal("Failed to get public key: ", err)
		}
	}
	s.Log("Deletion of keys by prefix verified.")
}
