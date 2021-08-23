// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilcoextension

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// NewConnectionToWilcoExtension returns a chrome driver connection to Wilco test extension.
func NewConnectionToWilcoExtension(ctx context.Context, cr *chrome.Chrome) (*wilcoConn, error) {
	bgURL := chrome.ExtensionBackgroundPageURL(ID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to extension at %s", bgURL)
	}
	return &wilcoConn{conn}, nil
}

// wilcoConn is a thin wrapper around the chrome.Conn pointer to add certain wilco specific methods.
type wilcoConn struct {
	*chrome.Conn
}

// CreatePort create a port to Wilco built-in application.
func (w *wilcoConn) CreatePort(ctx context.Context) error {
	if err := w.Eval(ctx, `const port = chrome.runtime.connectNative('com.google.wilco_dtc');`, nil); err != nil {
		return errors.Wrap(err, "failed to run javascript")
	}
	return nil
}

// StartListener starts receiving messages from the built-in messaging port.
func (w *wilcoConn) StartListener(ctx context.Context) error {
	if err := w.Eval(ctx, `
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
	});`, nil); err != nil {
		return errors.Wrap(err, "failed to run javascript")
	}
	return nil
}

// AddReply sets message as the reply to the next message. Multiple replies can be queued.
func (w *wilcoConn) AddReply(ctx context.Context, message interface{}) error {
	marshaledMessage, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}
	if err := w.Eval(ctx, fmt.Sprintf(`replies.push(%s);`, string(marshaledMessage)), nil); err != nil {
		return errors.Wrap(err, "failed to run javascript")
	}
	return nil
}

// WaitForMessage reads a messasge the built-in messaging port and waits if none are available.
func (w *wilcoConn) WaitForMessage(ctx context.Context, message interface{}) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var length int
		if err := w.Eval(ctx, `requests.length`, &length); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get number of requests"))
		}

		if length == 0 {
			return errors.New("no requests present")
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 30 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to wait for incoming requests")
	}

	// Extracting the message.
	if err := w.Eval(ctx, `requests.pop()`, message); err != nil {
		return errors.Wrap(err, "failed to get the top message")
	}
	return nil
}

// SendMessageAndGetReply sends a message over the built-in messaging port.
// It waits for the response to arrive and saves it in the response parameter.
func (w *wilcoConn) SendMessageAndGetReply(ctx context.Context, message, response interface{}) error {
	marshaled, err := json.Marshal(&message)
	if err != nil {
		return errors.Wrapf(err, "failed to marshall %v", message)
	}

	if err := w.Eval(ctx, fmt.Sprintf(`new Promise(function(resolve, reject) {
		chrome.runtime.sendNativeMessage('com.google.wilco_dtc', %s, function(response) {
			if (!response) {
				reject('No response')
			} else {
				resolve(response)
			}
		})
	})`, string(marshaled)), response); err != nil {
		return errors.Wrap(err, "failed to send message over buit-in messaging port")
	}

	return nil
}
