// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that cryptohome recovery process succeeds with testing server mediation",
		Contacts:     []string{"anastasiian@chromium.org", "cros-lurs@google.com"},
		SoftwareDeps: []string{"chrome", "tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

type cryptohomeRecoveryData struct {
	ReauthProofToken string `json:"reauthProofToken"`
	AccessToken      string `json:"accessToken"`
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
	s.Log("Fetch epoch from the server")
	epoch, err := fetchEpoch(client, data.AccessToken)
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
	response, err := mediate(client, data.AccessToken, request)
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

func fetchEpoch(client *http.Client, accessToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", fetchEpochURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/x-protobuf")

	return completeRequest(client, req)
}

func mediate(client *http.Client, accessToken string, request []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", mediateURL, bytes.NewBuffer(request))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Add("Content-Type", "application/x-protobuf")

	return completeRequest(client, req)
}

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
