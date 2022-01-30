// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserutil implements convenience utils needed to take actions with ash-chrome, lacros-chrome or browser instances.
// TODO: can't move this to browser package due to cyclic imports.
package browserutil

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

// CloseAboutBlank finds a first target that is about:blank, closes it, then waits until it's gone.
// TODO: Add browser.Type to Browser.
func CloseAboutBlank(ctx context.Context, br *browser.Browser, bt browser.Type) error {
	return CloseWindow(ctx, br, bt, chrome.BlankURL)
}

// CloseWindow finds a first target browser that matches a given url, closes it, then waits until it's gone.
// This doesn't depend on chrome.autotestPrivate unlike ash.CloseWindow.
func CloseWindow(ctx context.Context, br *browser.Browser, bt browser.Type, url string) error {
	if len(url) == 0 {
		return errors.New("url should not be empty")
	}

	targets, err := br.FindTargets(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return errors.Wrapf(err, "failed to query for about:blank pages in browser %v", bt)
	}
	if len(targets) == 0 {
		return errors.New("no matching target found")
	}

	allPages, err := br.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	if err != nil {
		return errors.Wrapf(err, "failed to query for all pages in browser %v", bt)
	}
	// Check if not all pages are being closed for lacros-chrome, otherwise the process will exit when the last window is closed.
	// Return an error to prevent it from not being shut down properly.
	if bt == browser.TypeLacros && len(allPages) == 1 {
		return errors.Wrap(err, "closing the last window will terminate the lacros-chrome. Instead, call the closeBrowser if browserfixt.SetUp is used or conn.Close() to release browser resources properly")
	}

	// Close a target window, and wait for it to be closed.
	err = br.CloseTarget(ctx, targets[0].TargetID)
	if err != nil {
		return errors.Wrapf(err, "failed to close a window in browser %v ", bt)
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		current, err := br.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
		if err != nil {
			return testing.PollBreak(err)
		}
		testing.ContextLogf(ctx, "hyungtaekim: CloseAboutBlank current: %v, previous: %v", len(current), len(targets))
		if len(current) != len(targets)-1 {
			return errors.New("not all about:blank targets were closed")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
}
