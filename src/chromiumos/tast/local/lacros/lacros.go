// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/testing"
)

// ChromeType indicates which type of Chrome browser to be used.
type ChromeType string

const (
	// ChromeTypeChromeOS indicates we are using the ChromeOS system's Chrome browser.
	ChromeTypeChromeOS ChromeType = "chromeos"
	// ChromeTypeLacros indicates we are using lacros-chrome.
	ChromeTypeLacros ChromeType = "lacros"
)

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

// CloseAboutBlank finds all targets that are about:blank and closes them.
func CloseAboutBlank(ctx context.Context, ds *cdputil.Session) error {
	targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	for _, info := range targets {
		ds.CloseTarget(ctx, info.TargetID)
	}
	return nil
}

func waitForWindowWithPredicate(ctx context.Context, ctconn *chrome.TestConn, p func(*ash.Window) bool) (*ash.Window, error) {
	if err := ash.WaitForCondition(ctx, ctconn, p, pollOptions); err != nil {
		return nil, err
	}
	return ash.FindWindow(ctx, ctconn, p)
}

// FindFirstBlankWindow finds the first window whose title is 'about:blank'.
func FindFirstBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "about:blank")
	})
}

// FindFirstNonBlankWindow finds the first window whose title is not 'about:blank'.
func FindFirstNonBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return !strings.Contains(w.Title, "about:blank")
	})
}
