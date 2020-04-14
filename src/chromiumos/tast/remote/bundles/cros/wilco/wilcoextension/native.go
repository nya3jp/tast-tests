// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilcoextension

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

// NativeMessaging is a helper to interact with the Wilco native messaging API
type NativeMessaging struct {
	pc ps.PolicyServiceClient
}

// NewNativeMessaging creates a new instance of NativeMessaging and connects to
// the native API.
func NewNativeMessaging(ctx context.Context, pc ps.PolicyServiceClient) (*NativeMessaging, error) {
	startScript := `
	const port = chrome.runtime.connectNative('com.google.wilco_dtc');`

	if _, err := pc.EvalStatementInExtension(ctx, &ps.EvalInExtensionRequest{
		ExtensionId: ID,
		Expression:  startScript,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create port to native application")
	}

	return &NativeMessaging{
		pc,
	}, nil
}

// SendMessage sends a message over the native messaging port.
func (n *NativeMessaging) SendMessage(ctx context.Context, message interface{}) error {
	marshaled, err := json.Marshal(&message)
	if err != nil {
		return errors.Wrapf(err, "failed to marshall %v", message)
	}

	if _, err := n.pc.EvalStatementInExtension(ctx, &ps.EvalInExtensionRequest{
		ExtensionId: ID,
		Expression:  fmt.Sprintf(`port.postMessage(%s)`, string(marshaled)),
	}); err != nil {
		return errors.Wrap(err, "failed to send native message")
	}

	return nil
}

// StartListener starts receiving messages from the native messaging port.
func (n *NativeMessaging) StartListener(ctx context.Context) error {
	startScript := `
	var requests = new Array();
	var replies = new Array();
	chrome.runtime.onConnectNative.addListener(function(port) {
		if (port.sender.nativeApplication !== 'com.google.wilco_dtc')
			return;
		
		port.onMessage.addListener(function(msg) {
			requests.push(msg);

			if (replies.length > 0) {
				port.postMessage(replies.pop());
			} else {
				port.disconnect();
			}
		})
	});`

	if _, err := n.pc.EvalStatementInExtension(ctx, &ps.EvalInExtensionRequest{
		ExtensionId: ID,
		Expression:  startScript,
	}); err != nil {
		return errors.Wrap(err, "failed to create port to native application")
	}

	return nil
}

// AddReply sets message as the reply to the next message. Multiple replies can be queued.
func (n *NativeMessaging) AddReply(ctx context.Context, message interface{}) error {
	marshaledMessage, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if _, err := n.pc.EvalStatementInExtension(ctx, &ps.EvalInExtensionRequest{
		ExtensionId: ID,
		Expression:  fmt.Sprintf(`replies.push(%s);`, string(marshaledMessage)),
	}); err != nil {
		return errors.Wrap(err, "failed to create port to native application")
	}

	return nil
}

// GetMessage reads a messasge the native messaging port.
func (n *NativeMessaging) GetMessage(ctx context.Context, message interface{}) error {
	res, err := n.pc.EvalInExtension(ctx, &ps.EvalInExtensionRequest{
		ExtensionId: ID,
		Expression:  `requests.pop()`,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create port to native application")
	}

	if err := json.Unmarshal(res.Result, message); err != nil {
		return errors.Wrap(err, "failed to unmarshal message")
	}

	return nil
}

// WaitForMessage reads a messasge the native messaging port and waits if none
// are available.
func (n *NativeMessaging) WaitForMessage(ctx context.Context, message interface{}) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		res, err := n.pc.EvalInExtension(ctx, &ps.EvalInExtensionRequest{
			ExtensionId: ID,
			Expression:  `requests.length`,
		})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get number of requests"))
		}

		var length int
		if err := json.Unmarshal(res.Result, &length); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to unmarshal request"))
		}

		if length == 0 {
			return errors.New("no requests present")
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 30 * time.Second,
	}); err != nil {
		return err
	}

	return n.GetMessage(ctx, message)
}
