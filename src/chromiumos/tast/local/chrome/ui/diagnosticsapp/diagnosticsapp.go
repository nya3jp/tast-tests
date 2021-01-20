// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diagnosticsapp contains drivers for controlling the ui of diagnostics SWA.
package diagnosticsapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

var diagnosticsRootNodeParams = ui.FindParams{
	Name: apps.Diagnostics.Name,
	Role: ui.RoleTypeWindow,
}

var diagnosticsLogButton = ui.FindParams{
	ClassName: "session-log-button",
	Role:      ui.RoleTypeButton,
}

// DiagnotsicsRootNode returns the root ui node of Diagnotsics app.
func DiagnotsicsRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, diagnosticsRootNodeParams, 20*time.Second)
}

// WaitForApp waits for the app to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	dxRootnode, err := DiagnotsicsRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find diagnostics app")
	}
	defer dxRootnode.Release(ctx)

	// Find the session log button to verify app is rendering.
	if _, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsLogButton, time.Minute); err != nil {
		return errors.Wrap(err, "failed to render diagnostics app")
	}
	return nil
}
