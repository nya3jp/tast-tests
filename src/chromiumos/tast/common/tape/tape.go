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

	"chromiumos/tast/ctxutil"
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

// requests is a struct representing requests for the setPolicy call to the TAPE server.
type requests struct {
	PolicyTargetKey policyTargetKey `json:"policyTargetKey"`

	PolicyValue policyValue `json:"policyValue"`
	UpdateMask  updateMask  `json:"updateMask"`
}

// policyTargetKey is part of the requests struct and holds the orgunit in which a policy
// will be set.
type policyTargetKey struct {
	TargetResource string `json:"targetResource"`
}

// policyValue is part of the requests struct and holds the uri to the PolicySchema and the values
// which will be set by the setPolicy call.
type policyValue struct {
	PolicySchema string      `json:"policySchema"`
	Value        interface{} `json:"value"`
}

// updateMask is part of the requests struct and holds the names of the parameters which will be set.
type updateMask struct {
	Paths []string `json:"paths"`
}

// PolicySchema is an interface for a more specific policy schema.  All the
// concrete policy schemas in this package must implement this interface.
type PolicySchema interface {
	// MarshalJSON takes a string representing an orgunit as argument and creates the JSON representation
	// of the PolicySchema used in a setPolicy call on the TAPE server. The orgunit identifies the
	// orgunit the policy should be set for. Every Account in TAPE has its own orgunit to prevent tests
	// from interfering with each other.
	MarshalJSON(string) ([]byte, error)
}

func marshalJSON(orgunit, policySchemaURI string, policySchema interface{}, updatePaths []string) ([]byte, error) {
	return json.Marshal(&struct {
		Requests requests `json:"requests"`
	}{
		Requests: requests{
			PolicyTargetKey: policyTargetKey{
				TargetResource: "orgunits/" + orgunit,
			},
			PolicyValue: policyValue{
				PolicySchema: policySchemaURI,
				Value:        policySchema,
			},
			UpdateMask: updateMask{
				Paths: updatePaths,
			},
		},
	})
}

// sendRequest makes a call to the specified REST endpoint of TAPE with the given http method and payload.
func sendRequest(ctx context.Context, method, endpoint string, payload *bytes.Reader) (*http.Response, error) {
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
// after the given timeout is reached. A timeout of <=0 will use the DEFAULT_ACCOUNT_TIMEOUT of the TAPE server
// which is 2 hours.
// The returned password is temporary and will be valid for roughly one day.
func RequestAccount(ctx context.Context, timeout time.Duration) (*Account, error) {
	timeoutSeconds := timeout * time.Second
	payloadBytes, err := json.Marshal(timeoutSeconds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	// Shorten the context to 30 seconds as the call should take longer.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	response, err := sendRequest(ctx, "POST", "requestAccount", payload)
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

	// Shorten the context to 30 seconds as the call should take longer.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	response, err := sendRequest(ctx, "POST", "regeneratePassword", payload)
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

	// Shorten the context to 60 seconds as the call should take longer.
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()
	response, err := sendRequest(ctx, "POST", "cleanUp", payload)
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

	// Shorten the context to 30 seconds as the call should take longer.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	response, err := sendRequest(ctx, "POST", "releaseAccount", payload)
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
	payloadBytes, err := policySchema.MarshalJSON(acc.Orgunit)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	// Shorten the context to 30 seconds as the call should take longer.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	response, err := sendRequest(ctx, "POST", "setPolicy", payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()

	return nil
}
