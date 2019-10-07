// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// QueryResponse contains a subset of the results returned from the Assistant server
// when it received a query. This struct contains the only fields which are used in
// the tests.
type QueryResponse struct {
	// Fallback contains text messages used as the "fallback" for HTML card rendering.
	// Generally the fallback text is similar to transcribed TTS, e.g. "It's exactly 6
	// o'clock." or "Turning bluetooth off.".
	Fallback string `json:"htmlFallback"`
}

// QueryStatus contains a subset of the status of an interaction with Assistant started
// by sending a query, e.g. query text, mic status, and query response. This struct contains
// the only fields which are used in the tests.
//
// TODO(meilinw): Add a reference for the struct after the API change landed (crrev.com/c/1552293).
type QueryStatus struct {
	// TODO(meilinw): Remove this entry once we replace with new autotest private API.
	Fallback      string `json:"htmlFallback"`
	QueryResponse `json:"queryResponse"`
}

// Enable brings up Google Assistant service and returns any errors.
func Enable(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		  chrome.autotestPrivate.setAssistantEnabled(true,
		      10 * 1000 /* timeout_ms */,
		      () => {
		        if (chrome.runtime.lastError === undefined) {
		          resolve();
		        } else {
		          reject(chrome.runtime.lastError.message);
		        }
		      });
		  })`, nil)
}

// SendTextQuery sends text query to Assistant and returns the query status.
func SendTextQuery(ctx context.Context, tconn *chrome.Conn, query string) (QueryStatus, error) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
		  chrome.autotestPrivate.sendAssistantTextQuery(%q,
		      10 * 1000 /* timeout_ms */,
		      function(status) {
		        if (chrome.runtime.lastError === undefined) {
		          resolve(status);
		        } else {
		          reject(chrome.runtime.lastError.message);
		        }
		      });
		  })`, query)

	var status QueryStatus
	err := tconn.EvalPromise(ctx, expr, &status)
	return status, err
}

// WaitForServiceReady checks the Assistant service readiness after enabled by waiting
// for a simple query interaction being completed successfully. Before b/129896357 gets
// resolved, it should be used to verify the service status before the real test starts.
func WaitForServiceReady(ctx context.Context, tconn *chrome.Conn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := SendTextQuery(ctx, tconn, "What's the time?")
		return err
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}
