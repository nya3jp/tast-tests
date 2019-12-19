// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
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
	return tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.setAssistantEnabled)(true, 10 * 1000 /* timeout_ms */)`, nil)
}

// EnableAndWaitForReady brings up Google Assistant service, waits for
// NEW_READY signal and returns any errors.
func EnableAndWaitForReady(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.enableAssistantAndWaitForReady)()`, nil)
}

// SendTextQuery sends text query to Assistant and returns the query status.
func SendTextQuery(ctx context.Context, tconn *chrome.Conn, query string) (QueryStatus, error) {
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.sendAssistantTextQuery)(%q, 10 * 1000 /* timeout_ms */)`, query)
	var status QueryStatus
	err := tconn.EvalPromise(ctx, expr, &status)
	return status, err
}

// WaitForServiceReady checks the Assistant service readiness after enabled by waiting
// for a simple query interaction being completed successfully. Before b/129896357 gets
// resolved, it should be used to verify the service status before the real test starts.
func WaitForServiceReady(ctx context.Context, tconn *chrome.Conn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := SendTextQuery(ctx, tconn, "1+1=")
		return err
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// SetHotwordEnabled turns on/off "OK Google" hotword detection for Assistant.
func SetHotwordEnabled(ctx context.Context, tconn *chrome.Conn, enabled bool) error {
	const prefName string = "settings.voice_interaction.hotword.enabled"
	expr := fmt.Sprintf(
		`tast.promisify(chrome.autotestPrivate.setWhitelistedPref)('%s', %t)`, prefName, enabled)
	return tconn.EvalPromise(ctx, expr, nil)
}

// ToggleUIWithHotkey mimics the Assistant key press to open/close the Assistant UI.
func ToggleUIWithHotkey(ctx context.Context, tconn *chrome.Conn) error {
	const accelerator = "{keyCode: 'assistant', shift: false, control: false, alt: false, search: false, pressed: true}"
	expr := fmt.Sprintf(
		`(async () => {
		   var accel = %s;
		   // Triggers the hotkey pressed event.
		   await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accel);
		   // Releases the key for cleanup. This release event will not be handled, so we ignore the result.
		   accel.pressed = false;
		   chrome.autotestPrivate.activateAccelerator(accel, () => {});
		 })()`, accelerator)

	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}

	return nil
}
