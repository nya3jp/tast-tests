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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

// TestParam holds parameters of window arrangement cuj test variations.
type TestParam struct {
	BrowserType browser.Type
	Tablet      bool
	Tracing     bool
	Validation  bool
}

// ChromeCleanUpFunc defines the clean up function of chrome browser.
type ChromeCleanUpFunc func(ctx context.Context) error

// CloseAboutBlankFunc defines the helper to close about:blank page. It is
// implemented for lacros-chrome and no-op for ash-chrome.
type CloseAboutBlankFunc func(ctx context.Context) error

// SetupChrome creates ash-chrome or lacros-chrome based on test parameters.
func SetupChrome(ctx context.Context, s *testing.State) (*chrome.Chrome, ash.ConnSource, *chrome.TestConn, ChromeCleanUpFunc, CloseAboutBlankFunc, *chrome.TestConn, error) {
	testParam := s.Param().(TestParam)

	var cr *chrome.Chrome
	var cs ash.ConnSource
	var l *lacros.Lacros
	var bTconn *chrome.TestConn

	cleanup := func(ctx context.Context) error { return nil }
	closeAboutBlank := func(ctx context.Context) error { return nil }

	if testParam.BrowserType == browser.TypeAsh {
		if testParam.Tablet {
			var err error
			if cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
				return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to init chrome")
			}
			cleanup = func(ctx context.Context) error {
				return cr.Close(ctx)
			}
		} else {
			cr = s.FixtValue().(*chrome.Chrome)
		}
		cs = cr

		var err error
		bTconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to get TestAPIConn")
		}
	} else {
		var err error
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), testParam.BrowserType)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to setup lacros")
		}
		cleanup = func(ctx context.Context) error {
			lacros.CloseLacros(ctx, l)
			return nil
		}

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to get lacros TestAPIConn")
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, errors.Wrap(err, "failed to conect to test api")
	}

	if testParam.BrowserType == browser.TypeLacros {
		closeAboutBlank = func(ctx context.Context) error {
			return l.CloseAboutBlank(ctx, tconn, 0)
		}
	}
	return cr, cs, tconn, cleanup, closeAboutBlank, bTconn, nil
}
