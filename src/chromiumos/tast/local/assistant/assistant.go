// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// QueryResponse contains a subset of the results returned from the Assistant server
// when it received a query.
type QueryResponse struct {
	// Fallback contains text messages used as the "fallback" for HTML card rendering.
	// Generally the fallback text is similar to transcribed TTS, e.g. "It's exactly 6
	// o'clock." or "Turning bluetooth off.".
	Fallback string `json:"htmlFallback"`
}

// QueryStatus contains a subset of the status of an interaction with Assistant started
// by sending a query, e.g. query text, mic status, and query response.
type QueryStatus struct {
	// TODO(meilinw):
	// Remove this entry once we replace with new autotest private API.
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
