// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"

	// "github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CreateWindows finds a first target browser that matches a given url, closes it, then waits until it's gone.
// This doesn't depend on chrome.autotestPrivate unlike ash.CloseWindow.
// TODO: Embed browser.Type in Browser struct, so it doesn't need to be passed in as a separate arg.

// CreateWindows create up to n number of browser windows with specified URL and wait for them to become visible.
// If a window with the URL is already opened before this fuction is called, it will be counted
// If there is already opened window with the URL, it will be counted.

// It will fail and return an error if at least one request fails to fulfill. Note that this will
// parallelize the requests to create windows, which may be bad if the caller
// wants to measure the performance of Chrome. This should be used for a
// preparation, before the measurement happens.
func CreateWindows(ctx context.Context, br *Browser, bt Type, url string, n int, opts ...CreateTargetOption) error {
	if len(url) == 0 {
		return errors.New("url should not be empty")
	}
	testing.ContextLog(ctx, "n: ", n)

	targets, err := br.FindTargets(ctx, MatchTargetURL(url))
	if err != nil {
		return errors.Wrapf(err, "failed to query for web pages with url (%v) in browser %v", url, bt)
	}
	if len(targets) > 0 {
		testing.ContextLogf(ctx, "already found %v of matching windows with url: %v", len(targets), url)
	}

	// allPages, err := br.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	// if err != nil {
	// 	return errors.Wrapf(err, "failed to query for all pages in browser %v", bt)
	// }

	// Open a target window, and wait for it to be open.
	// targetID := targets[0].TargetID
	// TODO: return conn
	if _, err := br.NewConn(ctx, url, opts...); err != nil {
		return errors.Wrapf(err, "failed to close a window in browser %v ", bt)
	}
	// return testing.Poll(ctx, func(ctx context.Context) error {
	// 	match, err := br.IsTargetAvailable(ctx, MatchTargetID(targetID))
	// 	if err != nil {
	// 		return testing.PollBreak(err)
	// 	}
	// 	if match {
	// 		return errors.Errorf("target was not closed, url: %v", url)
	// 	}
	// 	return nil
	// }, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
	return nil
}
