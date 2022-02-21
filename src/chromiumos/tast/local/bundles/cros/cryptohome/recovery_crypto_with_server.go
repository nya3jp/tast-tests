// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecoveryCryptoWithServer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cryptohome recovery process succeeds with testing server mediation. Currently the test runs only on TPM 2",
		Contacts:     []string{"anastasiian@chromium.org", "cros-lurs@google.com"},
		SoftwareDeps: []string{"chrome", "tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
			"cryptohome.RecoveryCryptoWithServer.accessTokenURL",
		},
	})
}

// cryptohomeRecoveryData stores tokens needed for cryptohome recovery testing.
// See https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl
type cryptohomeRecoveryData struct {
	ReauthProofToken string `json:"reauthProofToken"`
	RefreshToken     string `json:"refreshToken"`
}

// accessTokenFetchResponse is the data returned from the request to
// https://www.googleapis.com/oauth2/v4/token.
type accessTokenFetchResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresInSec int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

const (
	// Links to the test server.
	fetchEpochURL = "https://staging-chromeoslogin-pa.sandbox.googleapis.com/v1/epoch/1"
	mediateURL    = "https://staging-chromeoslogin-pa.sandbox.googleapis.com/v1/cryptorecovery"
)

func RecoveryCryptoWithServer(ctx context.Context, s *testing.State) {
	testTool, newErr := cryptohome.NewRecoveryTestTool()
	if newErr != nil {
		s.Fatal("Failed to initialize RecoveryTestTool", newErr)
	}
	defer func(s *testing.State, testTool *cryptohome.RecoveryTestTool) {
		if err := testTool.RemoveDir(); err != nil {
			s.Error("Failed to remove dir: ", err)
		}
	}(s, testTool)

	s.Log("Step 1 - create HSM payload")
	if err := testTool.CreateHsmPayload(ctx); err != nil {
		s.Fatal("Failed to execute CreateHsmPayload: ", err)
	}

	// Go through the OOBE flow to get the tokens.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.EnableFeatures("CryptohomeRecoveryFlow"),
		chrome.ExtraArgs("--force-cryptohome-recovery-for-testing"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Fetch the tokens from Chrome")
	var data cryptohomeRecoveryData
	if err := tconn.Call(ctx, &data, "tast.promisify(chrome.autotestPrivate.getCryptohomeRecoveryData)"); err != nil {
		s.Fatal("Failed to fetch tokens from Chrome: ", err)
	}
	testTool.SaveCustomRAPT([]byte(data.ReauthProofToken))

	client := &http.Client{}
	accessToken, err := fetchAccessTokenForRecovery(s, client, data.RefreshToken)
	if err != nil {
		s.Fatal("Failed to fetch access token: ", err)
	}

	s.Log("Fetch epoch from the server")
	epoch, err := fetchEpoch(client, accessToken)
	if err != nil {
		s.Fatal("Failed to fetch epoch: ", err)
	}
	testTool.SaveCustomEpoch(epoch)

	s.Log("Step 2 - create recovery request")
	if err := testTool.CreateRecoveryRequest(ctx); err != nil {
		s.Fatal("Failed to execute CreateRecoveryRequest: ", err)
	}

	request, err := testTool.GetRecoveryRequest()
	if err != nil {
		s.Fatal("Failed to get recovery request: ", err)
	}

	s.Log("Step 3 - mediate with the test server")
	response, err := mediate(client, accessToken, request)
	if err != nil {
		s.Fatal("Failed to mediate the request: ", err)
	}
	testTool.SaveCustomResponse(response)

	s.Log("Step 4 - decrypt the data")
	if err := testTool.Decrypt(ctx); err != nil {
		s.Fatal("Failed to execute Decrypt: ", err)
	}

	if err := testTool.Validate(ctx); err != nil {
		s.Fatal("Failed to validate: ", err)
	}
}

// fetchAccessTokenForRecovery makes a request to
// https://www.googleapis.com/oauth2/v4/token with the provided refreshToken
// and returns the access token.
func fetchAccessTokenForRecovery(s *testing.State, client *http.Client, refreshToken string) (string, error) {
	// Note: the URL contains Chrome client_id and client_secret.
	url := fmt.Sprintf("%s&refresh_token=%s", s.RequiredVar("cryptohome.RecoveryCryptoWithServer.accessTokenURL"), refreshToken)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create http request")
	}
	req.Header.Add("Content-Type", "application/json")

	respBody, err := completeRequest(client, req)
	if err != nil {
		return "", errors.Wrap(err, "failed to complete request")
	}

	var respData accessTokenFetchResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response")
	}

	return respData.AccessToken, nil
}

// fetchEpoch makes a request to `fetchEpochURL` with the provided accessToken
// and returns the response body if the response status is `http.StatusOK`.
func fetchEpoch(client *http.Client, accessToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", fetchEpochURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/x-protobuf")

	return completeRequest(client, req)
}

// mediate makes a request to `mediateURL` with the provided accessToken and
// request and returns the response body if the response status is
// `http.StatusOK`.
func mediate(client *http.Client, accessToken string, request []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", mediateURL, bytes.NewBuffer(request))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/x-protobuf")

	return completeRequest(client, req)
}

// completeRequest executes HTTP request with the provided client and returns
// the response body if the response status is `http.StatusOK`.
func completeRequest(client *http.Client, request *http.Request) ([]byte, error) {
	resp, err := client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send the request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed with status %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	return body, nil
}
