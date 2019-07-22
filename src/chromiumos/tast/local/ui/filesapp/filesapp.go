// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// ID is the Files App ID.
	ID = "hhaomjibdihmijegdhdafkllkbggdgoj"

	// DownloadPath is the location of Downloads for the test user.
	DownloadPath = "/home/chronos/user/Downloads/"

	// RoleButton is the chrome.automation role for buttons.
	RoleButton = "button"
	// RoleRootWebArea is the chrome.automation role for the root of a window.
	RoleRootWebArea = "rootWebArea"
	// RoleStaticText is the chrome.automation role for static text.
	RoleStaticText = "staticText"
)

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	ctx   context.Context
	tconn *chrome.Conn
}

// NewFilesApp launches the Files App and returns it.
// An error is returned if the app fails to launch.
func NewFilesApp(ctx context.Context, tconn *chrome.Conn) (*FilesApp, error) {
	f := &FilesApp{ctx: ctx, tconn: tconn}
	// Launch Files App.
	launchQuery := fmt.Sprintf("new Promise((onFulFilled) => {chrome.autotestPrivate.launchApp(%q, onFulFilled)})", ID)
	if err := tconn.EvalPromise(ctx, launchQuery, nil); err != nil {
		return nil, err
	}
	// Wait for Files App to be open.
	if err := f.WaitForElement(RoleRootWebArea, "Files - My files", time.Minute); err != nil {
		return nil, err
	}
	return f, nil
}

// OpenDownloadsFolder opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloadsFolder() error {
	// Find the Downloads label.
	if err := f.WaitForElement(RoleStaticText, "Downloads", 10*time.Second); err != nil {
		return err
	}
	// Click Downloads to open the folder.
	if err := f.ClickElement(RoleStaticText, "Downloads"); err != nil {
		return err
	}
	// Ensure the Files App has switched to the Downloads folder.
	return f.WaitForElement("rootWebArea", "Files - Downloads", 10*time.Second)
}

// ClickElement clicks on the element with the specific role and name.
// If the JavaScript fails to execute, an error is returned.
func (f *FilesApp) ClickElement(role string, name string) error {
	if err := f.initChromeAutomationRoot(); err != nil {
		return err
	}
	clickQuery := fmt.Sprintf(`var element = root.find({attributes: {role: %q, name: %q}});
							   element.doDefault();`, role, name)
	if err := f.tconn.Exec(f.ctx, clickQuery); err != nil {
		f.logRoleDebugInfo(role)
		return err
	}
	return nil
}

// WaitForElement waits for an element to exist.
// If the timeout is reached, an error is returned.
func (f *FilesApp) WaitForElement(role string, name string, timeout time.Duration) error {
	if err := f.initChromeAutomationRoot(); err != nil {
		return err
	}
	findQuery := fmt.Sprintf("root.find({attributes: {role: %q, name: %q}})", role, name)
	if err := f.tconn.WaitForExprWithTimeoutFailOnErr(f.ctx, findQuery, timeout); err != nil {
		f.logRoleDebugInfo(role)
		return err
	}
	return nil
}

// initChromeAutomationRoot sets up chrome.automation root for later calls.
// If the initilizing root fails or takes too long, an error is returned.
func (f *FilesApp) initChromeAutomationRoot() error {
	initQuery := "var root; chrome.automation.getDesktop(r => root = r);"
	if err := f.tconn.Exec(f.ctx, initQuery); err != nil {
		return err
	}
	// Wait for root to be ready.
	return f.tconn.WaitForExprWithTimeoutFailOnErr(f.ctx, "root", 10*time.Second)
}

// logRoleDebugInfo logs all elements with a role.
func (f *FilesApp) logRoleDebugInfo(role string) {
	var elements []string
	findQuery := fmt.Sprintf("root.findAll({attributes: {role: %q}}).map(node => node.name)", role)
	if err := f.tconn.Eval(f.ctx, findQuery, &elements); err != nil {
		testing.ContextLogf(f.ctx, "Failed to grab debug info for {role: %s}: %s", role, err)
		return
	}
	testing.ContextLogf(f.ctx, "Debug info for %s: %+q", role, elements)
}
