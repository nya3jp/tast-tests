// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
)

// TestApp represents the Play Billing test PWA and ARC Payments Overlay.
type TestApp struct {
	cr          *chrome.Chrome
	pbconn      *chrome.Conn
	tconn       *chrome.TestConn
	uiAutomator *ui.Device
}

const (
	appID              = "dlbmfdiobcnhnfocmenonncepnmhpckd"
	localServerAddress = "http://127.0.0.1/"
)

// NewTestApp returns a reference to a new Play Billing Test App.
func NewTestApp(ctx context.Context, cr *chrome.Chrome, arc *arc.ARC, uiAutomator *ui.Device) (*TestApp, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting Test API connection")
	}
	defer tconn.Close()

	return &TestApp{
		cr:          cr,
		tconn:       tconn,
		uiAutomator: uiAutomator,
		pbconn:      nil,
	}, nil
}

// Launch starts a new TestApp window.
func (ta *TestApp) Launch(ctx context.Context) error {
	if err := apps.Launch(ctx, ta.tconn, appID); err != nil {
		return errors.Wrapf(err, "failed launching app ID %q", appID)
	}

	pbconn, err := ta.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(localServerAddress))
	if err != nil {
		return errors.Wrapf(err, "failed getting connection for target: %q", localServerAddress)
	}

	ta.pbconn = pbconn

	return nil
}
