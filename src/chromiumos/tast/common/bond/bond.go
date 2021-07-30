// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bond provides the interface to access Bond API.
package bond

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/testing"
)

const (
	endpoint         = "https://bond-pa.sandbox.googleapis.com"
	hangoutEndpoints = "https://preprod-hangouts.googleapis.com/hangouts/v1_meetings/"
	meetingEndpoints = "https://preprod-meetings.sandbox.googleapis.com"

	scope = "https://www.googleapis.com/auth/meetings"

	defaultCredPath = "/creds/service_accounts/bond_service_account.json"
)

type newClientOption struct {
	credsJSON []byte
}

// NewClientOption is an option to customize creating a new client.
type NewClientOption func(*newClientOption)

// WithCredsJSON specifies the customized json credential rather than the
// default one.
func WithCredsJSON(jsonData []byte) NewClientOption {
	return func(opt *newClientOption) {
		opt.credsJSON = jsonData
	}
}

// Client is a client to send Bond API requests.
type Client struct {
	client *http.Client
}

// NewClient creates a new instance of Client.
func NewClient(ctx context.Context, opts ...NewClientOption) (*Client, error) {
	option := newClientOption{}
	for _, opt := range opts {
		opt(&option)
	}
	if len(option.credsJSON) == 0 {
		var err error
		if option.credsJSON, err = ioutil.ReadFile(defaultCredPath); err != nil {
			return nil, errors.Wrap(err, "failed to read the credential file")
		}
	}
	creds, err := google.CredentialsFromJSON(ctx, option.credsJSON, scope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the credentials")
	}
	return &Client{client: oauth2.NewClient(ctx, creds.TokenSource)}, nil
}

// Close closes the all unused connections.
func (c *Client) Close() {
	c.client.CloseIdleConnections()
}

func (c *Client) send(ctx context.Context, method, url string, reqObj, respObj interface{}) error {
	var bodyReader io.Reader
	if reqObj != nil {
		reqBytes, err := json.Marshal(reqObj)
		if err != nil {
			return errors.Wrap(err, "failed to marshal the request")
		}
		bodyReader = bytes.NewBuffer(reqBytes)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return errors.Wrap(err, "failed to create a new request")
	}
	req = req.WithContext(ctx)
	req.Header.Add("content-type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.Errorf("request failed with status code %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		return errors.Wrap(err, "failed to decode the response")
	}
	return nil
}

func (c *Client) sendWithRetry(ctx context.Context, method, url string, reqObj, respObj interface{}) error {
	const (
		retry         = 3
		retryInterval = 500 * time.Millisecond
	)
	return action.Retry(retry, func(ctx context.Context) error {
		return c.send(ctx, method, url, reqObj, respObj)
	}, retryInterval)(ctx)
}

// AvailableWorkers returns the number of available workers in the server.
func (c *Client) AvailableWorkers(ctx context.Context) (int, error) {
	type availableWorkersResponse struct {
		NumOfAvailableWorkers int `json:"numOfAvailableWorkers"`
	}
	resp := availableWorkersResponse{}
	if err := c.sendWithRetry(ctx, http.MethodGet, endpoint+"/v1/workers:count", nil, &resp); err != nil {
		return 0, err
	}
	return resp.NumOfAvailableWorkers, nil
}

// CreateConference creates a new conference and returns its meeting code.
func (c *Client) CreateConference(ctx context.Context) (string, error) {
	type conference struct {
		ConferenceCode string `json:"conferenceCode"`
	}
	type conferenceResponse struct {
		Conference *conference `json:"conference"`
	}
	req := map[string]interface{}{
		"conference_type": "THOR",
		"backend_options": map[string]string{
			"mesi_apiary_url":      hangoutEndpoints,
			"mas_one_platform_url": meetingEndpoints,
		},
	}
	resp := conferenceResponse{}
	if err := c.sendWithRetry(ctx, http.MethodPost, endpoint+"/v1/conferences:create", req, &resp); err != nil {
		return "", err
	}
	return resp.Conference.ConferenceCode, nil
}

// ExecuteScript requests the server to execute a script.
func (c *Client) ExecuteScript(ctx context.Context, script, meetingCode string) error {
	req := map[string]interface{}{
		"script": script,
		"conference": map[string]string{
			"conference_code": meetingCode,
		},
	}
	resp := map[string]interface{}{}
	if err := c.sendWithRetry(ctx, http.MethodPost, endpoint+"/v1/conference/"+meetingCode+"/script", req, &resp); err != nil {
		return err
	}
	if success, ok := resp["success"]; ok && success.(bool) {
		return nil
	}
	testing.ContextLogf(ctx, "Failed to execute script: error %#v", resp)
	return errors.New("failed to execute script")
}

type addBotsOptions struct {
	sendFPS         int
	requestedLayout string
	allowVP9        bool
	sendVP9         bool
}

// AddBotsOption customizes the request of AddBods.
type AddBotsOption func(*addBotsOptions)

// WithSendFPS can change the frame rate a bot produces.
func WithSendFPS(fps int) AddBotsOption {
	return func(opts *addBotsOptions) {
		opts.sendFPS = fps
	}
}

// WithLayout modifies the layout of the bot.
func WithLayout(layout string) AddBotsOption {
	return func(opts *addBotsOptions) {
		opts.requestedLayout = layout
	}
}

// WithVP9 customizes the VP9 capability of the bot.
func WithVP9(allow, send bool) AddBotsOption {
	return func(opts *addBotsOptions) {
		opts.allowVP9 = allow
		opts.sendVP9 = send
	}
}

// AddBots add a number of bots to the specified conference room with the
// duration. When succeeds, it also returns the list of bot IDs.
func (c *Client) AddBots(ctx context.Context, meetingCode string, numBots int, ttl time.Duration, opts ...AddBotsOption) ([]int, error) {
	options := addBotsOptions{
		sendFPS:         24,
		requestedLayout: "BRADY_BUNCH_4_4",
		allowVP9:        true,
		sendVP9:         true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	type addBotsResponse struct {
		BotIDs []int `json:"botIds"`
	}
	req := map[string]interface{}{
		"num_of_bots": numBots,
		"ttl_secs":    ttl / time.Second,
		"video_call_options": map[string]bool{
			"allow_vp9": options.allowVP9,
			"send_vp9":  options.sendVP9,
		},
		"media_options": map[string]interface{}{
			"audio_file_path":  "audio_32bit_48k_stereo.raw",
			"mute_audio":       true,
			"video_fps":        options.sendFPS,
			"mute_video":       false,
			"requested_layout": options.requestedLayout,
		},
		"backend_options": map[string]string{
			"mesi_apiary_url":      hangoutEndpoints,
			"mas_one_platform_url": meetingEndpoints,
		},
		"conference": map[string]string{
			"conference_code": meetingCode,
		},
		"bot_type":                           "MEETINGS",
		"use_random_video_file_for_playback": true,
	}
	resp := addBotsResponse{}
	if err := c.sendWithRetry(ctx, http.MethodPost, endpoint+"/v1/conference/"+meetingCode+"/bots:add", req, &resp); err != nil {
		return nil, err
	}
	return resp.BotIDs, nil
}
