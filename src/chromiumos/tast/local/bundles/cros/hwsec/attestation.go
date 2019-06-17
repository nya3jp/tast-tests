// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	b64 "encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/local/chrome"
	libhwsec "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	defaultPreparationForEnrolmentTimeout int = 40 * 1000
	defaultTakingOwnershipTimeout         int = 40 * 1000
)

const (
	defaultACAType     int                    = 0
	defaultCertProfile apb.CertificateProfile = apb.CertificateProfile_ENTERPRISE_USER_CERTIFICATE
	defaultCertOrigin  string                 = ""
	defaultCertLabel   string                 = "aaa"
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

func Attestation(ctx context.Context, s *testing.State) {
	s.Log("Start test with creating a proxy")
	utility, err := libhwsec.NewUtility(ctx, libhwsec.CryptohomeProxyLegacyType)
	if err != nil {
		s.Error("Utilty creation error: ", err)
		return
	}
	if err := libhwsec.EnsureTpmIsReady(utility, defaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
	}
	s.Log("Tpm is ensured to be ready")
	if err := libhwsec.EnsureIsPreparedForEnrollment(
		utility, defaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
	}
	s.Log("Attestation is prepared for enrollment")
	// For now, just uses '0' as in "default CA"
	req, err := utility.CreateEnrollRequest(defaultACAType)
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
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

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(resp, username, defaultCertLabel)

	if err != nil {
		s.Error("Failed to finish cert request: ", err)
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
	//TODO(cylai): verification of signed challenge; for now it is still not
	//clear about its priority because of the ROI and potential issue e.g. need
	//to talk to VA server via Internet.
	_, err = utility.SignEnterpriseVAChallenge(
		0,
		username,
		defaultCertLabel,
		username,
		"fake_device_id",
		true,
		challenge)
	if err != nil {
		s.Error("Failed to sign VA challenge: ", err)
	}
	s.Log("Challenge signed")
}

func sendPostRequestTo(body string, serverURL string) (string, error) {
	req, err := http.NewRequest("POST", serverURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

func sendGetRequestTo(serverURL string) (string, error) {
	resp, err := http.Get(serverURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

func decodeBase64String(enc string) ([]byte, error) {
	return b64.StdEncoding.DecodeString(enc)
}

func encodeToBase64String(dec []byte) string {
	return b64.StdEncoding.EncodeToString(dec)
}
