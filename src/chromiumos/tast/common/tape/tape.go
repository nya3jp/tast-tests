// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tape enables access to the TAPE service which offers access to owned test accounts and
// configuration of policies on DPanel for those accounts.
// The TAPE project is documented here: TODO(alexanderhartl): add link once its finished
package tape

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/errors"
)

const tapeURL = "https://tape-307412.ey.r.appspot.com/"

// Account is a struct representing an owned test account with its credentials.
type Account struct {
	Email    string `json:"email"`
	GaiaID   int64  `json:"gaiaid"`
	Orgunit  string `json:"orgunit"`
	Password string `json:"password"`
	Timeout  int64  `json:"timeout"`
}

// sendRequestWithTimeout makes a call to the specified REST endpoint of TAPE with the given http method and payload.
func sendRequestWithTimeout(ctx context.Context, method, endpoint string, timeout time.Duration, payload *bytes.Reader) (*http.Response, error) {
	// Shorten the context as the call should not take longer than timeout.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a request.
	req, err := http.NewRequestWithContext(ctx, method, tapeURL+endpoint, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request.
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a response from TAPE")
	}

	// Check if the call was successful.
	if response.StatusCode != 200 {
		return nil, errors.Errorf("%s at %s returned %d", method, endpoint, response.StatusCode)
	}

	return response, nil
}

// RequestAccount calls TAPE to obtain credentials for an available owned test account and returns it.
// The returned Account can not be obtained by other calls to RequestAccount until it is released or it times out
// after the given timeout is reached. A timeout of 0 will use the DEFAULT_ACCOUNT_TIMEOUT of the TAPE server
// which is 2 hours, timeouts larger than 2 days = 172800 seconds are not allowed.
// The returned password is temporary and will be valid for roughly one day.
func RequestAccount(ctx context.Context, timeout time.Duration) (*Account, error) {
	const maxTimeout = 172800 * time.Second
	if timeout > maxTimeout {
		return nil, errors.Errorf("timeout may not be larger than %v seconds", maxTimeout.Seconds())
	}

	payloadBytes, err := json.Marshal(timeout.Seconds())
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := sendRequestWithTimeout(ctx, "POST", "requestAccount", 30*time.Second, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	// Read the response.
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response from TAPE")
	}

	var acc Account
	err = json.Unmarshal([]byte(respBody), &acc)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}

// RegeneratePassword calls TAPE to obtain a new temporary Password for the given Account and returns the Account.
// A temporary password will be valid for roughly a day. Tests are not expected to use this as RequestAccount will
// already provide a temporary password.
func (acc *Account) RegeneratePassword(ctx context.Context) error {
	payloadBytes, err := json.Marshal(acc)

	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := sendRequestWithTimeout(ctx, "POST", "regeneratePassword", 30*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	// Read the response.
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response from TAPE")
	}

	var password string
	err = json.Unmarshal([]byte(respBody), &password)
	if err != nil {
		return err
	}
	acc.Password = password

	return nil
}

// CleanUp calls TAPE to clean up the Account. All policies will be set to their default values and all other
// state changes will also be reverted. However the account will not be released with this call.
// Accounts will always be cleaned up when they are requested with RequestAccount, tests are not expected to call
// this function to clean up when a test is finished.
func (acc *Account) CleanUp(ctx context.Context) error {
	payloadBytes, err := json.Marshal(acc)
	if err != nil {
		return errors.Wrap(err, "failed to convert Account to json")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := sendRequestWithTimeout(ctx, "POST", "cleanUp", 60*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	// Read the response.
	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response from TAPE")
	}

	return nil
}

// ReleaseAccount calls TAPE to release the Account so it becomes available again.
func (acc *Account) ReleaseAccount(ctx context.Context) error {
	payloadBytes, err := json.Marshal(acc)
	if err != nil {
		return errors.Wrap(err, "failed to convert Account to json")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := sendRequestWithTimeout(ctx, "POST", "releaseAccount", 30*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	// Read the response.
	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response from TAPE")
	}

	return nil
}

// SetPolicy calls TAPE to set policySchema in DPanel.
func (acc *Account) SetPolicy(ctx context.Context, policySchema PolicySchema) error {
	payloadBytes, err := policySchema.Schema2JSON(acc.Orgunit)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := sendRequestWithTimeout(ctx, "POST", "setPolicy", 30*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	return nil
}
