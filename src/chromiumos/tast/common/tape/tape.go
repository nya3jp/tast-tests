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
	// Return the Oauth client using the credentials.
	ts, err := createTokenSource(ctx, credsJSON)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token")
	}

	client := &client{
		httpClient: oauth2.NewClient(ctx, ts),
	}

	return client, nil
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
	// Send the request.
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a response from TAPE")
	}
	// Check if the call was successful.
	if response.StatusCode != 200 {
		return nil, errors.Errorf("%s at %s returned %d", method, endpoint, response.StatusCode)
	}
	return response, nil
}

type requestAccountParams interface {
	endpoint() string
	validate() error
}

func validateAccountRequest(timeoutInSeconds, maxTimeoutInSeconds int64, poolID *string) error {
	// Validate the provided parameters.
	if timeoutInSeconds > maxTimeoutInSeconds {
		return errors.Errorf("Timeout may not be larger than %v seconds", maxTimeoutInSeconds)
	}

	if poolID != nil && len(*poolID) <= 0 {
		return errors.New("PoolID must not be empty when set")
	}

	return nil
}

func (c *client) requestAccount(ctx context.Context, params requestAccountParams) ([]byte, error) {
	if err := params.validate(); err != nil {
		return nil, errors.Wrap(err, "validation of the account request failed")
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}
	payload := bytes.NewReader(payloadBytes)

	response, err := c.sendRequestWithTimeout(ctx, "POST", params.endpoint(), 30*time.Second, payload)
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

func (rp *requestGenericAccountParams) endpoint() string {
	return "GenericAccount/request"
}

func (rp *requestGenericAccountParams) validate() error {
	return validateAccountRequest(int64(rp.TimeoutInSeconds), int64(MaxGenericAccountTimeoutInSeconds), rp.PoolID)
}

// NewRequestGenericAccountParams creates a new requestGenericAccountParams struct filled with the given parameters.
func NewRequestGenericAccountParams(timeoutInSeconds int32, poolID string) *requestGenericAccountParams {
	return &requestGenericAccountParams{
		TimeoutInSeconds: timeoutInSeconds,
		PoolID:           &poolID,
	}
}

// RequestGenericAccount sends a request for leasing a generic account and returns the account in a GenericAccount struct.
func (c *client) RequestGenericAccount(ctx context.Context, params *requestGenericAccountParams) (*GenericAccount, error) {
	respBody, err := c.requestAccount(ctx, params)
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

func (rp *requestOwnedTestAccountParams) endpoint() string {
	return "OTA/request"
}

func (rp *requestOwnedTestAccountParams) validate() error {
	return validateAccountRequest(int64(rp.TimeoutInSeconds), int64(MaxOwnedTestAccountTimeout), rp.PoolID)
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
func (c *client) RequestOwnedTestAccount(ctx context.Context, params *requestOwnedTestAccountParams) (*OwnedTestAccount, error) {
	respBody, err := c.requestAccount(ctx, params)
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

// NewSetPolicyRequest creates a setPolicyRequest struct for use in the SetPolicy function.
func NewSetPolicyRequest(policySchema PolicySchema, orgunitID, customerID string) (*setPolicyRequest, error) {
	schemaJSONString, err := policySchema.Schema2JSON(orgunitID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}

	request := &setPolicyRequest{
		PolicySchema: string(schemaJSONString),
		CustomerID:   customerID,
	}
	return request, nil
}

// SetPolicy calls TAPE to set a policySchema in DPanel.
func (c *client) SetPolicy(ctx context.Context, request *setPolicyRequest) error {
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
	DeviceID   string `json:"device_id"`
	CustomerID string `json:"customer_id"`
}

// NewDeprovisionRequest creates a DeprovisionRequest for use in the Deprovision function.
func NewDeprovisionRequest(deivceID, customerID string) *deprovisionRequest {
	request := &deprovisionRequest{
		DeviceID:   deivceID,
		CustomerID: customerID,
	}
	return request
}

// Deprovision calls TAPE to deprovision a device in DPanel.
func (c *client) Deprovision(ctx context.Context, request *deprovisionRequest) error {
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
