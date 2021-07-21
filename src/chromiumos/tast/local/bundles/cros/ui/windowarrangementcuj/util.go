// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package windowarrangementcuj contains helper util and test code for
// WindowArrangementCUJ.
package windowarrangementcuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// ChromeCleanUpFunc defines the clean up function of chrome browser.
type ChromeCleanUpFunc func(ctx context.Context) error

// SetupChrome creates ash-chrome or lacros-chrome based on test parameters.
func SetupChrome(ctx context.Context, s *testing.State, tablet bool) (ash.ConnSource, *chrome.TestConn, ChromeCleanUpFunc, error) {
	var cr *chrome.Chrome
	var cs ash.ConnSource

	cleanup := func(ctx context.Context) error { return nil }

	if tablet {
		var err error
		if cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to init chrome")
		}
		cleanup = func(ctx context.Context) error {
			return cr.Close(ctx)
		}
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}
	cs = cr

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to conect to test api")
	}

	return cs, tconn, cleanup, nil
}
