// Copyright 2020 The ChromiumOS Authors
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

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// InternalEndpoint - for internal use.
	InternalEndpoint = "https://bond-pa.sandbox.googleapis.com"
	// ExternalEndpoint - for external use.
	ExternalEndpoint = "https://botsondemand.googleapis.com"

	hangoutEndpoints = "https://hangouts.googleapis.com/hangouts/v1_meetings/"
	meetingEndpoints = "https://preprod-meetings.googleapis.com"

	scope = "https://www.googleapis.com/auth/meetings"

	defaultCredPath = "/creds/service_accounts/bond_service_account.json"
)

type newClientOption struct {
	credsJSON []byte
	endpoint  string
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

// WithEndpoint specifies a customized endpoint.
func WithEndpoint(endpoint string) NewClientOption {
	return func(opt *newClientOption) {
		opt.endpoint = endpoint
	}
}

// WithInternalEndpoint specifies the internal endpoint InternalEndpoint.
func WithInternalEndpoint() NewClientOption {
	return func(opt *newClientOption) {
		opt.endpoint = InternalEndpoint
	}
}

// WithExternalEndpoint specifies the external endpoint ExternalEndpoint.
func WithExternalEndpoint() NewClientOption {
	return func(opt *newClientOption) {
		opt.endpoint = ExternalEndpoint
	}
}

// Client is a client to send Bond API requests.
type Client struct {
	client   *http.Client
	endpoint string
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

	var endpoint string
	if option.endpoint != "" {
		endpoint = option.endpoint
	} else {
		endpoint = InternalEndpoint
	}

	return &Client{client: oauth2.NewClient(ctx, creds.TokenSource), endpoint: endpoint}, nil
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
		retry           = 3
		retryInterval   = 500 * time.Millisecond
		exponentialBase = 10
	)
	return action.RetryWithExponentialBackoff(retry, func(ctx context.Context) error {
		return c.send(ctx, method, url, reqObj, respObj)
	}, retryInterval, exponentialBase)(ctx)
}

// AvailableWorkers returns the number of available workers in the server.
func (c *Client) AvailableWorkers(ctx context.Context) (int, error) {
	type availableWorkersResponse struct {
		NumOfAvailableWorkers int `json:"numOfAvailableWorkers"`
	}
	resp := availableWorkersResponse{}
	if err := c.sendWithRetry(ctx, http.MethodGet, c.endpoint+"/v1/workers:count", nil, &resp); err != nil {
		return 0, err
	}
	return resp.NumOfAvailableWorkers, nil
}

// Set of methods for internal BOND API:

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
	if err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/conferences:create", req, &resp); err != nil {
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
	if err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/conference/"+meetingCode+"/script", req, &resp); err != nil {
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
// duration. On success, it also returns the list of bot IDs and the number
// of bots that failed to join.
func (c *Client) AddBots(ctx context.Context, meetingCode string, numBots int, ttl time.Duration, opts ...AddBotsOption) ([]int, int, error) {
	var layout string

	if numBots <= 1 {
		layout = "SPOTLIGHT"
	} else if numBots <= 5 {
		layout = "BRADY_BUNCH"
	} else if numBots <= 15 {
		layout = "BRADY_BUNCH_4_4"
	} else {
		layout = "BRADY_BUNCH_7_7"
	}

	options := addBotsOptions{
		sendFPS:         24,
		requestedLayout: layout,
		allowVP9:        true,
		sendVP9:         true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	type addBotsResponse struct {
		NumberOfFailures int   `json:"numberOfFailures"`
		BotIDs           []int `json:"botIds"`
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
	if err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/conference/"+meetingCode+"/bots:add", req, &resp); err != nil {
		return nil, 0, err
	}
	return resp.BotIDs, resp.NumberOfFailures, nil
}

// RemoveAllBots removes all bots from the specified conference room. On success,
// it also returns the list of IDs of bots that failed to be removed and the
// list of IDs of bots that were not found.
func (c *Client) RemoveAllBots(ctx context.Context, meetingCode string) (failedIDs, notFoundIDs []int, err error) {
	type botState struct {
		BotID int `json:"botId"`
	}
	type removeAllBotsResponse struct {
		NotFound []botState `json:"notFound"`
		Failed   []botState `json:"failed"`
	}
	req := map[string]interface{}{
		"conference": map[string]string{
			"conference_code": meetingCode,
		},
		"bot_type":   "MEETINGS",
		"remove_all": true,
	}
	resp := removeAllBotsResponse{}
	if err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/conference/"+meetingCode+"/bots:remove", req, &resp); err != nil {
		return nil, nil, err
	}

	var failedBotIDs []int
	for _, state := range resp.Failed {
		failedBotIDs = append(failedBotIDs, state.BotID)
	}
	var notFoundBotIDs []int
	for _, state := range resp.NotFound {
		notFoundBotIDs = append(notFoundBotIDs, state.BotID)
	}
	return failedBotIDs, notFoundBotIDs, nil
}

// Set of methods accessible externally:
// https://developers.google.com/bots-on-demand/reference/rest/v1/createConferenceWithBots/create.html

// CreateConferenceWithBots creates a new conference and adds bots. Returns its meeting code and number of failed bots.
// https://developers.google.com/bots-on-demand/reference/rest/v1/createConferenceWithBots/create
func (c *Client) CreateConferenceWithBots(ctx context.Context, numBots int, ttl time.Duration) (string, int, error) {
	req := map[string]interface{}{
		"numOfBots": numBots,
		"ttlSecs":   ttl / time.Second,
	}
	type response struct {
		ConferenceCode string   `json:"conferenceCode"`
		ErrorMessages  []string `json:"errorMessages"`
	}
	resp := response{}

	err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/createConferenceWithBots", req, &resp)

	var nFailures int
	if resp.ErrorMessages != nil {
		nFailures = len(resp.ErrorMessages)
		testing.ContextLogf(ctx, "CreateConferenceWithBots failed to create some bots: %#v", resp)
	} else {
		nFailures = 0
	}
	return resp.ConferenceCode, nFailures, err
}

// RemoveAllBotsFromConference removes all bots from a conference. Returns number of failed bots.
// https://developers.google.com/bots-on-demand/reference/rest/v1/removeAllBotsFromConference/removeAllBotsFromConference.html
func (c *Client) RemoveAllBotsFromConference(ctx context.Context, conferenceCode string) (int, error) {
	type response struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	resp := response{}

	err := c.sendWithRetry(ctx, http.MethodPost, c.endpoint+"/v1/removeAllBotsFromConference/"+conferenceCode, nil, &resp)

	var nFailures int
	if resp.ErrorMessages != nil {
		nFailures = len(resp.ErrorMessages)
		testing.ContextLogf(ctx, "RemoveAllBotsFromConference failed to remove some bots: %#v", resp)
	} else {
		nFailures = 0
	}
	return nFailures, err
}
