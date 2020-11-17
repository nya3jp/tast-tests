// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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

// CloseAboutBlank finds all targets that are about:blank and closes them.
func CloseAboutBlank(ctx context.Context, ds *cdputil.Session) error {
	targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	for _, info := range targets {
		if err := ds.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}
	testing.Poll(ctx, func(ctx context.Context) error {
		targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(targets) != 0 {
			return errors.New("not all about:blank targets were closed")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
	return nil
}
