// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CallboxManagerClient is an HTTP client wrapper for making requests to a
// Callbox Manager service.
type CallboxManagerClient struct {
	baseURL        string
	defaultCallbox string
}

func (s CallboxManagerClient) sendRequest(ctx context.Context, method, pathFromBaseURL string, body io.Reader, queryParams, headers map[string]string) (*http.Response, error) {
	reqURL, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse url from baseURL %q", s.baseURL)
	}
	reqURL.Path = path.Join(reqURL.Path, pathFromBaseURL)

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}
	if queryParams != nil {
		q := req.URL.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}
	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send %s request to %q", method, req.URL)
	}
	if resp.StatusCode != 200 {
		// All endpoints should return 200 on success
		respBody := new(strings.Builder)
		_, _ = io.Copy(respBody, resp.Body)
		return resp, errors.Errorf("%s %q returned non-200 status code %d with body=%q", method, req.URL, resp.StatusCode, respBody)
	}
	return resp, nil
}

func (s CallboxManagerClient) sendJSONPost(ctx context.Context, pathFromBaseURL string, queryParams map[string]string, requestBody interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal requestBody to json: %v", requestBody)
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	testing.ContextLogf(ctx, "CallboxManager %s %q queryParams=%q json=%q", http.MethodPost, pathFromBaseURL, queryParams, jsonBody)
	return s.sendRequest(ctx, http.MethodPost, pathFromBaseURL, bytes.NewReader(jsonBody), queryParams, headers)
}

func (s CallboxManagerClient) sendJSONGet(ctx context.Context, pathFromBaseURL string, queryParams map[string]string, requestBody interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal requestBody to json: %v", requestBody)
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := s.sendRequest(ctx, http.MethodGet, pathFromBaseURL, bytes.NewReader(jsonBody), queryParams, headers)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send request to CallboxManager: %s %q queryParams=%q json=%q", http.MethodGet, pathFromBaseURL, queryParams, jsonBody)
	}

	return resp, nil
}

func unmarshalResponse(resp *http.Response, res interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return errors.Wrap(err, "failed to unmarshal callbox manager response")
	}

	return nil
}

// ConfigureCallbox configures a callbox using the settings specified in the requestBody.
//
// Note: With the current prototype version of the server, only one callbox may
// be configured at a time. If there is any error with this configuration, you
// must view the server logs directly as no useful information will be returned
// here aside from raising an error if the config fails.
func (s CallboxManagerClient) ConfigureCallbox(ctx context.Context, requestBody *ConfigureCallboxRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/config", nil, requestBody)
	return err
}

// BeginSimulation instructs the server's configured callbox to begin the simulation.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) BeginSimulation(ctx context.Context, requestBody *BeginSimulationRequestBody) error {
	if requestBody == nil {
		requestBody = &BeginSimulationRequestBody{}
	}
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/start", nil, requestBody)
	return err
}

// ConfigureTxPower sets the callbox Tx (uplink) power.
func (s CallboxManagerClient) ConfigureTxPower(ctx context.Context, requestBody *ConfigureTxPowerRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/config/power/uplink", nil, requestBody)
	return err
}

// ConfigureRxPower sets the callbox Rx (downlink) power.
func (s CallboxManagerClient) ConfigureRxPower(ctx context.Context, requestBody *ConfigureRxPowerRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/config/power/downlink", nil, requestBody)
	return err
}

// FetchTxPower queries the closed loop Tx (uplink) power set on the callbox.
func (s CallboxManagerClient) FetchTxPower(ctx context.Context, requestBody *FetchTxPowerRequestBody) (*FetchTxPowerResponseBody, error) {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}

	resp, err := s.sendJSONGet(ctx, "/config/fetch/power/uplink", nil, requestBody)
	if err != nil {
		return nil, err
	}

	var res FetchTxPowerResponseBody
	if err := unmarshalResponse(resp, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// FetchRxPower queries the Rx (downlink) power set on the callbox.
func (s CallboxManagerClient) FetchRxPower(ctx context.Context, requestBody *FetchRxPowerRequestBody) (*FetchRxPowerResponseBody, error) {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}

	resp, err := s.sendJSONGet(ctx, "/config/fetch/power/downlink", nil, requestBody)
	if err != nil {
		return nil, err
	}

	var res FetchRxPowerResponseBody
	if err := unmarshalResponse(resp, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// SendSms instructs the server's configured callbox to send an sms message.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) SendSms(ctx context.Context, requestBody *SendSmsRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/sms", nil, requestBody)
	return err
}

// ConfigureIperf configures an Iperf measurement on the callbox.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) ConfigureIperf(ctx context.Context, requestBody *ConfigureIperfRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/iperf/config", nil, requestBody)
	return err
}

// StartIperf starts an Iperf measurement on the callbox with the current configuration.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) StartIperf(ctx context.Context, requestBody *StartIperfRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/iperf/start", nil, requestBody)
	return err
}

// StopIperf stops any current Iperf measurement on the callbox.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) StopIperf(ctx context.Context, requestBody *StopIperfRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/iperf/stop", nil, requestBody)
	return err
}

// CloseIperf stops any current Iperf measurement on the callbox and releases any resources held open.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) CloseIperf(ctx context.Context, requestBody *CloseIperfRequestBody) error {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}
	_, err := s.sendJSONPost(ctx, "/iperf/close", nil, requestBody)
	return err
}

// FetchIperfResult fetches the current result from the Iperf measurement on the callbox, invalid results are set to nil.
//
// Before calling this method, configure the callbox with ConfigureCallbox.
func (s CallboxManagerClient) FetchIperfResult(ctx context.Context, requestBody *FetchIperfResultRequestBody) (*FetchIperfResultResponseBody, error) {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}

	resp, err := s.sendJSONGet(ctx, "/iperf/fetch/result", nil, requestBody)
	if err != nil {
		return nil, err
	}

	var res FetchIperfResultResponseBody
	if err := unmarshalResponse(resp, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// FetchIperfIP fetches the Iperf measurement (DAU) IP address.
func (s CallboxManagerClient) FetchIperfIP(ctx context.Context, requestBody *FetchIperfIPRequestBody) (*FetchIperfIPResponseBody, error) {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}

	resp, err := s.sendJSONGet(ctx, "/iperf/fetch/ip", nil, requestBody)
	if err != nil {
		return nil, err
	}

	var res FetchIperfIPResponseBody
	if err := unmarshalResponse(resp, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// FetchMaxThroughput fetches the maximum achievable throughput of the callbox given its current configuration (im Mbit/s).
func (s CallboxManagerClient) FetchMaxThroughput(ctx context.Context, requestBody *FetchMaxThroughputRequestBody) (*FetchMaxThroughputResponseBody, error) {
	if requestBody.Callbox == "" {
		requestBody.Callbox = s.defaultCallbox
	}

	resp, err := s.sendJSONGet(ctx, "/config/fetch/maxthroughput", nil, requestBody)
	if err != nil {
		return nil, err
	}

	var res FetchMaxThroughputResponseBody
	if err := unmarshalResponse(resp, &res); err != nil {
		return nil, err
	}

	return &res, nil
}
