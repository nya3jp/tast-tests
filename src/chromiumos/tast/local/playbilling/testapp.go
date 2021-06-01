// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"

	"chromiumos/tast/local/android/ui"
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

// NewTestApp returns a reference to a new Play Billing Test App.
func NewTestApp(ctx context.Context, cr *chrome.Chrome, arc *arc.ARC, uiAutomator *ui.Device) (*TestApp, error) {
	return &TestApp{
		cr:          cr,
		uiAutomator: uiAutomator,
		pbconn:      nil,
	}, nil
}
