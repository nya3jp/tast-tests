// Copyright 2022 The ChromiumOS Authors
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
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const tapeURL = "https://tape-307412.ey.r.appspot.com/"
const tapeAudience = "770216225211-ihjn20dlehf94m9l4l5h0b0iilvd1vhc.apps.googleusercontent.com"

// client is created with NewClient and holds a *http.Client struct with an oauth token
// for authentication against the TAPE GCP.
type client struct {
	httpClient *http.Client
}

// createTokenSource an oauth2.TokenSource from service account key credentials.
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

// NewClient creates a http client which provides the necessary oauth token to authenticate with the TAPE
// GCP from the service account credentials in credsJSON.
func NewClient(ctx context.Context, credsJSON []byte) (*client, error) {
	// Check if token content was written to the DUT and should be used.
	if _, err := os.Stat(dutTokenFilePath); err == nil {
		tokenBytes, err := os.ReadFile(dutTokenFilePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read token content")
		}

		var tokenContent oauth2.Token
		if err := json.Unmarshal(tokenBytes, &tokenContent); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal token content")
		}
		return &client{
			httpClient: oauth2.NewClient(ctx, oauth2.StaticTokenSource(&tokenContent)),
		}, nil
	}

	// Return the Oauth client using the supplied credentials.
	ts, err := createTokenSource(ctx, credsJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token from json")
	}

	return &client{
		httpClient: oauth2.NewClient(ctx, ts),
	}, nil
}

// sendRequestWithTimeout makes a call to the specified REST endpoint of TAPE with the given http method and payload.
func (c *client) sendRequestWithTimeout(ctx context.Context, method, endpoint string, timeout time.Duration, payload *bytes.Reader) (*http.Response, error) {
	// Set the timeout of the http client and return to the original after.
	originalTimeout := c.httpClient.Timeout
	c.httpClient.Timeout = timeout
	defer func() {
		c.httpClient.Timeout = originalTimeout
	}()

	// Create a request.
	req, err := http.NewRequestWithContext(ctx, method, tapeURL+endpoint, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Try to make the call 3 times as a call might fail occasionally.
	var response *http.Response
	for i := 0; i < 3; i++ {
		// Send the request.
		response, err = c.httpClient.Do(req)
		if err != nil {
			continue
		}
		// Check if the call was successful.
		if response.StatusCode != 200 {
			testing.ContextLogf(ctx, "%s at %s returned %s", method, endpoint, response.Status)
			continue
		}
		break
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to get a response from TAPE")
	}
	// Check if the call was successful.
	if response.StatusCode != 200 {
		responseBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, errors.Errorf("%s at %s returned %s failed to read response body", method, endpoint, response.Status)
		}
		return nil, errors.Errorf("%s at %s returned %s %s", method, endpoint, response.Status, string(responseBytes))
	}
	return response, nil
}

func validateAccountRequest(timeoutInSeconds, maxTimeoutInSeconds int64, poolID *string) error {
	// Validate the provided parameters.
	if timeoutInSeconds > maxTimeoutInSeconds {
		return errors.Errorf("Timeout may not be larger than %v seconds, got %v seconds", maxTimeoutInSeconds, timeoutInSeconds)
	}

	if poolID != nil && len(*poolID) <= 0 {
		return errors.New("PoolID must not be empty when set")
	}

	return nil
}

func (c *client) requestAccount(ctx context.Context, endpoint string, params interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := c.sendRequestWithTimeout(ctx, "POST", endpoint, 30*time.Second, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request")
	}
	defer response.Body.Close()

	// Read the response.
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	return respBody, nil
}

func (c *client) releaseAccount(ctx context.Context, account interface{}, endpoint string) error {
	payloadBytes, err := json.Marshal(account)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := c.sendRequestWithTimeout(ctx, "POST", endpoint, 30*time.Second, payload)
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

// DefaultAccountTimeoutInSeconds is the default timeout for an account request (5 mintues).
const DefaultAccountTimeoutInSeconds = 60 * 5

type requestAccountOption struct {
	TimeoutInSeconds int32
	PoolID           *string
}

// RequestAccountOption provides options for requesting an account.
type RequestAccountOption func(*requestAccountOption)

// WithTimeout provides the option to set a timeout for the account lease.
func WithTimeout(timeoutInSeconds int32) RequestAccountOption {
	return func(opt *requestAccountOption) {
		opt.TimeoutInSeconds = timeoutInSeconds
	}
}

// WithPoolID provides the option to set a poolID from which the account should be taken.
func WithPoolID(poolID string) RequestAccountOption {
	return func(opt *requestAccountOption) {
		opt.PoolID = &poolID
	}
}

// GenericAccount holds the data of a generic account that can be used in tests.
type GenericAccount struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	PoolID      string  `json:"pool_id"`
	ReleaseTime float64 `json:"release_time"`
	RequestID   string  `json:"request_id"`
}

// MaxGenericAccountTimeoutInSeconds is the maximum timeout which is allowed
// when requesting a generic account (2 hours).
const MaxGenericAccountTimeoutInSeconds = 60 * 60 * 2

// requestOwnedTestAccountParams is a struct containing the necessary data to request a generic
// account from TAPE.
type requestGenericAccountParams struct {
	TimeoutInSeconds int32   `json:"timeout"`
	PoolID           *string `json:"pool_id"`
}

// NewRequestGenericAccountParams creates a new requestGenericAccountParams struct filled with the given parameters.
func NewRequestGenericAccountParams(timeoutInSeconds int32, poolID string) *requestGenericAccountParams {
	return &requestGenericAccountParams{
		TimeoutInSeconds: timeoutInSeconds,
		PoolID:           &poolID,
	}
}

// RequestGenericAccount sends a request for leasing a generic account and returns the account in a GenericAccount struct.
func (c *client) RequestGenericAccount(ctx context.Context, opts ...RequestAccountOption) (*GenericAccount, error) {
	// Copy over all options.
	options := requestAccountOption{
		TimeoutInSeconds: DefaultAccountTimeoutInSeconds,
	}
	for _, opt := range opts {
		opt(&options)
	}
	params := requestGenericAccountParams(options)

	validateAccountRequest(int64(params.TimeoutInSeconds), int64(MaxGenericAccountTimeoutInSeconds), params.PoolID)

	respBody, err := c.requestAccount(ctx, "GenericAccount/request", params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request account")
	}

	var account GenericAccount
	err = json.Unmarshal(respBody, &account)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return &account, nil
}

// ReleaseGenericAccount sends a request for releasing a leased generic account.
func (c *client) ReleaseGenericAccount(ctx context.Context, account *GenericAccount) error {
	return c.releaseAccount(ctx, account, "GenericAccount/release")
}

// MaxOwnedTestAccountTimeout is the maximum timeout which is allowed
// when requesting an owned test account (6 hours).
const MaxOwnedTestAccountTimeout = 60 * 60 * 6

// OwnedTestAccount holds all data of an owned test account which can be used in tests.
type OwnedTestAccount struct {
	GenericAccount
	GaiaID     int64  `json:"gaia_id"`
	CustomerID string `json:"customer_id"`
	OrgunitID  string `json:"orgunit_id"`
}

// requestOwnedTestAccountParams is a struct containing the necessary data to request an owned
// test account from TAPE.
type requestOwnedTestAccountParams struct {
	requestGenericAccountParams
	Lock bool `json:"lock"`
}

// NewRequestOwnedTestAccountParams creates a new requestOwnedTestAccountParams struct filled with the given parameters.
func NewRequestOwnedTestAccountParams(timeoutInSeconds int32, poolID string, lock bool) *requestOwnedTestAccountParams {
	params := NewRequestGenericAccountParams(timeoutInSeconds, poolID)
	return &requestOwnedTestAccountParams{
		requestGenericAccountParams: *params,
		Lock:                        lock,
	}
}

// RequestOwnedTestAccount sends a request for leasing a generic account and returns the account in an OwnedTestAccount struct.
func (c *client) RequestOwnedTestAccount(ctx context.Context, lock bool, opts ...RequestAccountOption) (*OwnedTestAccount, error) {
	// Copy over all options.
	options := requestAccountOption{
		TimeoutInSeconds: DefaultAccountTimeoutInSeconds,
	}
	for _, opt := range opts {
		opt(&options)
	}

	params := requestOwnedTestAccountParams{
		requestGenericAccountParams: requestGenericAccountParams(options),
		Lock:                        lock,
	}

	validateAccountRequest(int64(params.TimeoutInSeconds), int64(MaxOwnedTestAccountTimeout), params.PoolID)

	respBody, err := c.requestAccount(ctx, "OTA/request", params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request account")
	}

	var account OwnedTestAccount
	err = json.Unmarshal([]byte(respBody), &account)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	return &account, nil
}

// ReleaseAccount calls TAPE to release the Account so it becomes available again.
func (c *client) ReleaseOwnedTestAccount(ctx context.Context, account *OwnedTestAccount) error {
	return c.releaseAccount(ctx, account, "OTA/release")
}

// setPolicyRequest is a struct containing the necessary data to set a policy schema in DPanel.
type setPolicyRequest struct {
	PolicySchema string `json:"policy_schema"`
	CustomerID   string `json:"customer_id"`
}

// SetPolicy calls TAPE to set a policySchema in DPanel.
func (c *client) SetPolicy(ctx context.Context, policySchema PolicySchema, orgunitID, customerID string) error {
	schemaJSONString, err := policySchema.Schema2JSON(orgunitID)
	if err != nil {
		return errors.Wrap(err, "failed to marshal policy schema")
	}

	request := &setPolicyRequest{
		PolicySchema: string(schemaJSONString),
		CustomerID:   customerID,
	}

	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := c.sendRequestWithTimeout(ctx, "POST", "Policies/setPolicy", 30*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()
	return nil
}

// deprovisionRequest is a struct containing the necessary data to deprovision a device.
type deprovisionRequest struct {
	DeviceID   string `json:"deviceid"`
	CustomerID string `json:"customerid"`
}

// Deprovision calls TAPE to deprovision a device in DPanel.
func (c *client) Deprovision(ctx context.Context, deviceID, customerID string) error {
	request := &deprovisionRequest{
		DeviceID:   deviceID,
		CustomerID: customerID,
	}

	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)
	response, err := c.sendRequestWithTimeout(ctx, "POST", "Devices/deprovision", 60*time.Second, payload)
	if err != nil {
		return errors.Wrap(err, "failed to make REST call")
	}
	defer response.Body.Close()
	return nil
}
