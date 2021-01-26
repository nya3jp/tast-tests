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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
)

var diagnosticsRootNodeParams = ui.FindParams{
	Name: apps.Diagnostics.Name,
	Role: ui.RoleTypeWindow,
}

// DxLogButton export is used to find session log button
var DxLogButton = ui.FindParams{
	ClassName: "session-log-button",
	Role:      ui.RoleTypeButton,
}

// DxCPUTestButton export is used to find routine test button
var DxCPUTestButton = ui.FindParams{
	Name: "Run CPU test",
	Role: ui.RoleTypeButton,
}

// DxViewReportButton export is used to find the see report button
var DxViewReportButton = ui.FindParams{
	Name: "See Report",
	Role: ui.RoleTypeButton,
}

// DxCancelTestButton export is used to find routine test cancel button
var DxCancelTestButton = ui.FindParams{
	Name: "Stop test",
	Role: ui.RoleTypeButton,
}

// DxCPUChart export is used to find the realtime cpu chart
var DxCPUChart = ui.FindParams{
	ClassName: "legend-group",
	Role:      ui.RoleTypeGenericContainer,
}

// DxSuccessBadge export is used to find success badge notification
var DxSuccessBadge = ui.FindParams{
	Name: "SUCCESS",
	Role: ui.RoleTypeStaticText,
}

// DxCancelledBadge export is used to find cancelled badge
var DxCancelledBadge = ui.FindParams{
	Name: "STOPPED",
	Role: ui.RoleTypeStaticText,
}

// DiagnosticsRootNode returns the root ui node of Diagnotsics app.
func DiagnosticsRootNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	return ui.FindWithTimeout(ctx, tconn, diagnosticsRootNodeParams, 20*time.Second)
}

// WaitForApp waits for the app to be shown and rendered.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn) error {
	dxRootnode, err := DiagnosticsRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find diagnostics app")
	}
	defer dxRootnode.Release(ctx)

	// Find the session log button to verify app is rendering.
	if _, err := dxRootnode.DescendantWithTimeout(ctx, DxLogButton, 3*time.Minute); err != nil {
		return errors.Wrap(err, "failed to render diagnostics app")
	}
	return nil
}

// Launch diagnostics via default method and return root node.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	err := apps.Launch(ctx, tconn, apps.Diagnostics.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch diagnostics app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.Diagnostics.ID)
	if err != nil {
		return nil, errors.Wrap(err, "diagnostics app did not appear in shelf after launch")
	}

	dxRootnode, err := DiagnosticsRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find diagnostics app")
	}
	return dxRootnode, nil
}
