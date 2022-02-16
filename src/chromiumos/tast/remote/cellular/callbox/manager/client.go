// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"bytes"
	"context"
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

func (s CallboxManagerClient) sendJSONPost(ctx context.Context, pathFromBaseURL string, queryParams map[string]string, requestBody RequestBody) (*http.Response, error) {
	// Marshal json
	jsonBody, err := requestBody.Marshall()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal requestBody to json: %v", requestBody)
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	testing.ContextLogf(ctx, "CallboxManager %s %q queryParams=%q json=%q", http.MethodPost, pathFromBaseURL, queryParams, jsonBody)
	return s.sendRequest(ctx, http.MethodPost, pathFromBaseURL, bytes.NewReader(jsonBody), queryParams, headers)
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
