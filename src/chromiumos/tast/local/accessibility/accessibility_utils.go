// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accessibilityutils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const (
	chromeVoxExtensionURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/chromevox/background/background.html"
)

// Feature represents an accessibility feature in Chrome OS.
type Feature string

// List of accessibility features.
const (
	SpokenFeedback  Feature = "spokenFeedback"
	SwitchAccess            = "switchAccess"
	SelectToSpeak           = "selectToSpeak"
	FocusHighlight          = "focusHighlight"
	DockedMagnifier         = "dockedMagnifier"
	ScreenMagnifier         = "screenMagnifier"
)

// Returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func chromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extConn, err := c.NewConnForTarget(ctx, chrome.MatchTargetURL(chromeVoxExtensionURL))
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't execute NewConnForTarget correctly")
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313.
	if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	if err := chrome.AddTastLibrary(ctx, extConn); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "failed to introduce tast library")
	}

	return extConn, nil
}

// Enables spoken feedback.
// A connection to the ChromeVox extension background page is returned, and this will be
// closed by the calling function.
func WaitForSpokenFeedbackReady(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	cvconn, err := chromeVoxExtConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "creating connection to ChromeVox extension failed: ")
	}

	// Poll until ChromeVox connection finishes loading.
	if err := cvconn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "timed out waiting for ChromeVox connection to be ready")
	}

	return cvconn, nil
}

// SetFeatureEnabled sets the specified accessibility feature enabled/disabled using the provided connection to the extension.
func SetFeatureEnabled(ctx context.Context, tconn *chrome.TestConn, feature Feature, enable bool) error {
	if err := tconn.Call(ctx, nil, `(feature, enable) => {
		  return tast.promisify(tast.bind(chrome.accessibilityFeatures[feature], "set"))({value: enable});
		}`, feature, enable); err != nil {
		return errors.Wrapf(err, "failed to toggle %v to %t", feature, enable)
	}
	return nil
}

func SetSpokenFeedbackEnabled(ctx context.Context, tconn *chrome.TestConn, enable bool) error {
	if err := tconn.Call(ctx, nil, `(enable) => {
		  return tast.promisify(tast.bind(chrome.accessibilityFeatures["spokenFeedback"], "set"))({value: enable});
		}`, enable); err != nil {
		return errors.Wrapf(err, "failed to toggle Spoken feedback to %t", enable)
	}
	return nil
}

// focusedNode returns the currently focused node of ChromeVox.
// The returned node should be release by the caller.
func focusedNode(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn) (*ui.Node, error) {
	obj := &chrome.JSObject{}
	if err := cvconn.Eval(ctx, "ChromeVoxState.instance.currentRange.start.node", obj); err != nil {
		return nil, err
	}
	return ui.NewNode(ctx, tconn, obj)
}

// WaitForFocusedNode polls until the properties of the focused node matches the given params.
// timeout specifies the timeout to use when polling.
func WaitForFocusedNode(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, params *ui.FindParams, timeout time.Duration) error {
	// Wait for focusClassName to receive focus.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		focused, err := focusedNode(ctx, cvconn, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer focused.Release(ctx)

		if match, err := focused.Matches(ctx, *params); err != nil {
			return testing.PollBreak(err)
		} else if !match {
			return errors.Errorf("focused node is incorrect: got %v, want %v", focused, params)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}
