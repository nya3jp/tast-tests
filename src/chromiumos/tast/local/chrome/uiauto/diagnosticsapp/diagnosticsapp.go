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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

var diagnosticsRootNodeParams = nodewith.Name(apps.Diagnostics.Name).Role(role.Window)

// DxLogButton export is used to find session log button
var DxLogButton = nodewith.ClassName("session-log-button").Role(role.Button)

// DxMemoryTestButton export is used to find routine test button
var DxMemoryTestButton = nodewith.Name("Run Memory test").Role(role.Button)

// DxCPUTestButton export is used to find routine test button
var DxCPUTestButton = nodewith.Name("Run CPU test").Role(role.Button)

// DxViewReportButton export is used to find the see report button
var DxViewReportButton = nodewith.Name("See Report").Role(role.Button)

// DxCancelTestButton export is used to find routine test cancel button
var DxCancelTestButton = nodewith.Name("Stop test").Role(role.Button)

// DxCPUChart export is used to find the realtime cpu chart
var DxCPUChart = nodewith.ClassName("legend-group").Role(role.GenericContainer)

// DxPassedBadge export is used to find success badge notification
var DxPassedBadge = nodewith.Name("PASSED").Role(role.StaticText)

// DxProgressBadge export is used to find successful launch of a routine
var DxProgressBadge = nodewith.Name("RUNNING").Role(role.StaticText)

// DxCancelledBadge export is used to find cancelled badge
var DxCancelledBadge = nodewith.Name("STOPPED").Role(role.StaticText)

// DxConnectivity export is used to find the Connectivity navigation item.
var DxConnectivity = nodewith.Name("Connectivity").Role(role.GenericContainer)

// DxNetworkList export is used to find the network list.
var DxNetworkList = nodewith.ClassName("diagnostics-network-list-container").Role(role.GenericContainer)

// DxInput export is used to find the Input navigation item.
var DxInput = nodewith.Name("Input").Role(role.GenericContainer)

// DxKeyboardList export is used to find the keyboard heading on the input page.
var DxKeyboardList = nodewith.Attribute("device-type", "keyboard")//.Role(role.GenericContainer)

// DxKeyboardHeading export is used to find the keyboard heading on the input page.
var DxKeyboardHeading = nodewith.Name("Keyboard").Role(role.StaticText)

// DiagnosticsRootNode returns the root ui node of Diagnotsics app.
func DiagnosticsRootNode(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	ui := uiauto.New(tconn)
	err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsRootNodeParams)(ctx)
	return diagnosticsRootNodeParams, err
}

// Launch diagnostics via default method and finder and error.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	err := apps.Launch(ctx, tconn, apps.Diagnostics.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch diagnostics app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.Diagnostics.ID, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "diagnostics app did not appear in shelf after launch")
	}

	dxRootnode, err := DiagnosticsRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find diagnostics app")
	}
	return dxRootnode, nil
}
