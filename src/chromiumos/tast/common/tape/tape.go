// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"chromiumos/tast/errors"
)

// MaxTimeout is the maximum timeout which is allowed when requesting a test account.
const MaxTimeout = 60 * 60 * 6

// MaxGenericAccountTimeoutInSeconds is the maximum timeout which is allowed
// when requesting a generic account (2 hours).
const MaxGenericAccountTimeoutInSeconds = 60 * 60 * 2

const tapeURL = "https://tape-307412.ey.r.appspot.com/"
const tapeAudience = "770216225211-ihjn20dlehf94m9l4l5h0b0iilvd1vhc.apps.googleusercontent.com"

// Account is a struct representing an owned test account with its credentials.
type Account struct {
	ID          int64   `json:"id"`
	UserName    string  `json:"username"`
	GaiaID      int64   `json:"gaia_id"`
	CustomerID  string  `json:"customer_id"`
	OrgunitID   string  `json:"orgunit_id"`
	Password    string  `json:"password"`
	PoolID      string  `json:"pool_id"`
	ReleaseTime float64 `json:"release_time"`
	RequestID   string  `json:"request_id"`
}

// RequestOTAParams holds the parameters for the
// request generic account endpoint.
type RequestOTAParams struct {
	TimeoutInSeconds int32   `json:"timeout"`
	PoolID           *string `json:"pool_id"`
	Lock             bool    `json:"lock"`
}

// GenericAccount stores information about a generic account in TAPE.
type GenericAccount struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	PoolID      string  `json:"pool_id"`
	ReleaseTime float64 `json:"release_time"`
	RequestID   string  `json:"request_id"`
}

// RequestGenericAccountParams holds the parameters for the
// request generic account endpoint.
type RequestGenericAccountParams struct {
	TimeoutInSeconds int32   `json:"timeout"`
	PoolID           *string `json:"pool_id"`
}

// createTokenSource an oauth2.TokenSource from a service account key file.
func createTokenSource(ctx context.Context, credsJSON []byte) (oauth2.TokenSource, error) {
	config, err := google.JWTConfigFromJSON(credsJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate JWT config")
	}
	customClaims := make(map[string]interface{})
	customClaims["target_audience"] = tapeAudience

	config.PrivateClaims = customClaims
	config.UseIDToken = true

	return config.TokenSource(ctx), nil
}

type newTapeClientOption struct {
	credsJSON       []byte
	pathToCredsJSON string
}

// NewTapeClientOption is used to configure the TAPE client.
type NewTapeClientOption func(*newTapeClientOption)

// WithCredsJSON specifies the json service account data to use.
func WithCredsJSON(jsonData []byte) NewTapeClientOption {
	return func(opt *newTapeClientOption) {
		opt.credsJSON = jsonData
	}
}

// WithPathToCredsJSON specifies the path to a service account file to use.
func WithPathToCredsJSON(pathToCredsJSON string) NewTapeClientOption {
	return func(opt *newTapeClientOption) {
		opt.pathToCredsJSON = pathToCredsJSON
	}
}

// NewTapeClient creates a http client which provides the necessary token to connect to the TAPE
// GCP from a service account key file. This function can only be called remotely as the DuT does
// not have service account key files. All functions in the tape package should be passed this http client.
func NewTapeClient(ctx context.Context, opts ...NewTapeClientOption) (*http.Client, error) {
	// Copy over all options.
	option := newTapeClientOption{}
	for _, opt := range opts {
		opt(&option)
	}

	// Load the correct credentials.
	var credsJSON []byte
	var err error
	if len(option.credsJSON) > 0 {
		credsJSON = option.credsJSON
	} else if len(option.pathToCredsJSON) > 0 {
		credsJSON, err = ioutil.ReadFile(option.pathToCredsJSON)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read credential json")
		}
	} else {
		return nil, errors.New("One of credsJSON or pathToCredsJSON must be set")
	}

	// Return the Oauth client using the credentials.
	ts, err := createTokenSource(ctx, credsJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token")
	}

	return oauth2.NewClient(ctx, ts), nil
}

// sendRequestWithTimeout makes a call to the specified REST endpoint of TAPE with the given http method and payload.
func sendRequestWithTimeout(ctx context.Context, method, endpoint string, timeout time.Duration, payload *bytes.Reader, client *http.Client) (*http.Response, error) {
	// Set the timeout of the client and return to the original after.
	originalTimeout := client.Timeout
	client.Timeout = timeout
	defer func() {
		client.Timeout = originalTimeout
	}()

	// Create a request.
	req, err := http.NewRequestWithContext(ctx, method, tapeURL+endpoint, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	// Send the request.
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
func RequestAccount(ctx context.Context, params RequestOTAParams, client *http.Client) (*Account, error) {
	// Validate the provided parameters.
	if int64(params.TimeoutInSeconds) > int64(MaxTimeout) {
		return nil, errors.Errorf("Timeout may not be larger than %v seconds", MaxTimeout)
	}

	if params.PoolID != nil && len(*params.PoolID) <= 0 {
		return nil, errors.New("PoolID must not be empty when set")
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "OTA/request", 30*time.Second, payload, client)
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
func (acc *Account) RegeneratePassword(ctx context.Context, client *http.Client) error {
	payloadBytes, err := json.Marshal(acc)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "regeneratePassword", 30*time.Second, payload, client)
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
func (acc *Account) CleanUp(ctx context.Context, client *http.Client) error {
	payloadBytes, err := json.Marshal(acc)
	if err != nil {
		return errors.Wrap(err, "failed to convert Account to json")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "cleanUp", 60*time.Second, payload, client)
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
func (acc *Account) ReleaseAccount(ctx context.Context, client *http.Client) error {
	payloadBytes, err := json.Marshal(acc)
	if err != nil {
		return errors.Wrap(err, "failed to convert Account to json")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "OTA/release", 30*time.Second, payload, client)
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
func (acc *Account) SetPolicy(ctx context.Context, policySchema PolicySchema, client *http.Client) error {
	payloadBytes, err := policySchema.Schema2JSON(acc.OrgunitId)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "Policies/setPolicy", 30*time.Second, payload, client)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()
	return nil
}

// DeprovisionRequest is a struct containing the necessary data to deprovision a device.
type DeprovisionRequest struct {
	DeviceID   string `json:"deviceid"`
	CustomerID string `json:"customerid"`
}

// Deprovision calls TAPE to deprovision a device in DPanel.
func Deprovision(ctx context.Context, request DeprovisionRequest, client *http.Client) error {
	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "Devices/deprovision", 60*time.Second, payload, client)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()
	return nil
}

// RequestGenericAccount sends a request for leasing a generic account.
func RequestGenericAccount(ctx context.Context, params RequestGenericAccountParams, client *http.Client) (*GenericAccount, error) {
	// Validate the provided parameters.
	if int64(params.TimeoutInSeconds) > int64(MaxGenericAccountTimeoutInSeconds) {
		return nil, errors.Errorf("Timeout may not be larger than %v seconds", MaxGenericAccountTimeoutInSeconds)
	}

	if params.PoolID != nil && len(*params.PoolID) <= 0 {
		return nil, errors.New("PoolID must not be empty when set")
	}

	// Make the request
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "GenericAccount/request", 30*time.Second, payload, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request")
	}
	defer response.Body.Close()

	// Read the response.
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}
	var acc GenericAccount
	if err := json.Unmarshal([]byte(respBody), &acc); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return &acc, nil
}

// ReleaseGenericAccount sends a request for releasing a leased account.
func ReleaseGenericAccount(ctx context.Context, account *GenericAccount, client *http.Client) error {
	if account == nil {
		return errors.New("account is not set")
	}

	if client == nil {
		return errors.New("client is not set")
	}

	// Make the request
	payloadBytes, err := json.Marshal(account)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := sendRequestWithTimeout(ctx, "POST", "GenericAccount/release", 30*time.Second, payload, client)
	if err != nil {
		return errors.Wrap(err, "failed to make request")
	}
	defer response.Body.Close()

	// Make sure the request was successful.
	if response.StatusCode != 200 {
		return errors.Errorf("failed to release account, status code: %d", response.StatusCode)
	}

	return nil
}
