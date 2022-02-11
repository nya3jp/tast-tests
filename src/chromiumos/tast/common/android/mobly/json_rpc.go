// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mobly is for interacting with Mobly snippets on Android devices for rich Android automation controls.
// See https://github.com/google/mobly-snippet-lib for more details.
package mobly

import (
	"bufio"
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DefaultRPCResponseTimeout is the default timeout for receiving an RPC response from the snippet.
// Most RPCs should return a response within a short amount of time.
// Some RPCs such as eventWaitAndGet may not respond until their specified timeouts are reached.
const DefaultRPCResponseTimeout = 10 * time.Second

// jsonRPCCmd is the command format required to initialize the RPC server.
type jsonRPCCmd struct {
	Cmd string `json:"cmd"`
	UID int    `json:"uid"`
}

// jsonRPCCmdResponse is the corresponding response format to jsonRPCCmd. Only used when initializing the server.
type jsonRPCCmdResponse struct {
	Status bool `json:"status"`
	UID    int  `json:"uid"`
}

// jsonRPCRequest is the primary request format for snippet RPCs.
type jsonRPCRequest struct {
	Method string        `json:"method"`
	ID     int           `json:"id"`
	Params []interface{} `json:"params"`
}

// JSONRPCResponse is the corresponding response format for jsonRPCRequest.
// The Result field's format varies depending on which method is called by
// the request, so it should be unmarshalled based on the request's API.
type JSONRPCResponse struct {
	ID       int             `json:"id"`
	Result   json.RawMessage `json:"result"`
	Callback string          `json:"callback"`
	Error    string          `json:"error"`
}

// clientSend writes a request to the RPC server. A newline is appended
// to the request body as it is required by the RPC server.
func (sc *SnippetClient) clientSend(body []byte) error {
	if _, err := sc.conn.Write(append(body, "\n"...)); err != nil {
		return errors.Wrap(err, "failed to write to server")
	}
	return nil
}

// clientReceive reads the RPC server's response.
func (sc *SnippetClient) clientReceive(timeout time.Duration) ([]byte, error) {
	sc.conn.SetReadDeadline(time.Now().Add(timeout))
	bufReader := bufio.NewReader(sc.conn)
	res, err := bufReader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return res, nil
}

// RPC formats the provided RPC method and arguments as a jsonRPCRequest, sends it to the server, and returns the server's response.
func (sc *SnippetClient) RPC(ctx context.Context, timeout time.Duration, method string, args ...interface{}) (*JSONRPCResponse, error) {
	// Create and send the request.
	reqID := sc.requestID
	request := jsonRPCRequest{ID: reqID, Method: method, Params: make([]interface{}, 0)}
	if len(args) > 0 {
		request.Params = args
	}
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request to json")
	}
	testing.ContextLog(ctx, "\tRPC request: ", string(requestBytes))

	if err := sc.clientSend(requestBytes); err != nil {
		return nil, err
	}
	sc.requestID++

	// Receive and process the response.
	var res JSONRPCResponse
	b, err := sc.clientReceive(timeout)
	testing.ContextLog(ctx, "\tRPC response: ", string(b))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	if res.Error != "" {
		return nil, errors.Errorf("response error %v", res.Error)
	}
	if res.ID != reqID {
		return nil, errors.Errorf("response ID mismatch; expected %v, got %v", reqID, res.ID)
	}
	return &res, nil
}

// initialize initializes the snippet.
func (sc *SnippetClient) initialize(ctx context.Context) error {
	// Initialize the snippet server. Running the 'initiate' command with uid -1 is necessary to create a new session to the server.
	reqCmd := jsonRPCCmd{UID: -1, Cmd: "initiate"}
	reqCmdBody, err := json.Marshal(&reqCmd)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal request (%+v) to json", reqCmd)
	}
	testing.ContextLog(ctx, "Initialize command request: ", string(reqCmdBody))
	if err := sc.clientSend(reqCmdBody); err != nil {
		return errors.Wrap(err, "failed to send initialize command")
	}
	b, err := sc.clientReceive(DefaultRPCResponseTimeout)
	testing.ContextLog(ctx, "Initialize command response: ", string(b))
	if err != nil {
		return errors.Wrap(err, "failed to read response to initialize command")
	}

	// Unmarshal the response and check if the initialize command was successful.
	var res jsonRPCCmdResponse
	if err := json.Unmarshal(b, &res); err != nil {
		return errors.Wrap(err, "failed to unmarshal initialize command response")
	}
	if !res.Status {
		return errors.New("snippet RPC initialize command did not succeed")
	}
	return nil
}

// EventWaitAndGet waits for the specified event associated with the RPC that returned callbackID to appear in the snippet's event cache.
func (sc *SnippetClient) EventWaitAndGet(ctx context.Context, callbackID, eventName string, timeout time.Duration) (*EventWaitAndGetResult, error) {
	// Read the response with a slightly extended timeout. `eventWaitAndGet` won't respond until the event is posted in the snippet cache,
	// or the timeout is reached. In the timeout case, we need to set the TCP read deadline a little later so we'll get the response before the conn times out.
	res, err := sc.RPC(ctx, timeout+time.Second, "eventWaitAndGet", callbackID, eventName, int(timeout.Milliseconds()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get eventWaitAndGet response")
	}
	// Sample response: {"callback_id":"1-1", "name":"eventName", "creation_time":"1642817334319", "data":{'key': 'value'}}
	var result EventWaitAndGetResult
	if err := json.Unmarshal(res.Result, &result); err != nil {
		return nil, errors.Wrap(err, "failed to read result map from json response")
	}
	return &result, nil
}

// EventWaitAndGetResult maps the 'result' field of EventWaitAndGet's JSONRPCResponse to a format that's easier to work with.
type EventWaitAndGetResult struct {
	CallbackID   int                    `json:"callback_id"`
	Name         string                 `json:"name"`
	CreationTime int                    `json:"creation_time"`
	Data         map[string]interface{} `json:"data"`
}
