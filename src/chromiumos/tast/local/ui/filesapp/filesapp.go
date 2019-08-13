// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/apps"
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

	// expectedWindowName is the expected webroot window name of the Files App on launch.
	expectedWindowName = "Files - My files"
)

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	tconn *chrome.Conn
}

// Launch launches the Files App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.Conn) (*FilesApp, error) {
	f := &FilesApp{tconn: tconn}
	// Launch the Files App.
	if err := apps.LaunchApp(ctx, tconn, ID); err != nil {
		return nil, err
	}
	// Wait for the Files App to be open.
	if err := f.WaitForElement(ctx, RoleRootWebArea, expectedWindowName, time.Minute); err != nil {
		return nil, errors.Wrapf(err, "failed to find element {role: %q, name: %q}", RoleRootWebArea, expectedWindowName)
	}
	return f, nil
}

// OpenDownloads opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads(ctx context.Context) error {
	// Find the Downloads label.
	if err := f.WaitForElement(ctx, RoleStaticText, "Downloads", 10*time.Second); err != nil {
		return err
	}
	// Click Downloads to open the folder.
	if err := f.ClickElement(ctx, RoleStaticText, "Downloads"); err != nil {
		return err
	}
	// Ensure the Files App has switched to the Downloads folder.
	return f.WaitForElement(ctx, RoleRootWebArea, "Files - Downloads", 10*time.Second)
}

// ClickElement clicks on the element with the specific role and name.
// If the JavaScript fails to execute, an error is returned.
func (f *FilesApp) ClickElement(ctx context.Context, role string, name string) error {
	clickQuery := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.find({attributes: {role: %q, name: %q}}).doDefault());", role, name)
	if err := f.tconn.EvalPromise(ctx, clickQuery, nil); err != nil {
		f.logRoleDebugInfo(ctx, role)
		return err
	}
	return nil
}

// WaitForElement waits for an element to exist.
// If the timeout is reached, an error is returned.
func (f *FilesApp) WaitForElement(ctx context.Context, role string, name string, timeout time.Duration) error {
	findQuery := fmt.Sprintf(
		`(async () => {
			const root = await tast.promisify(chrome.automation.getDesktop)();
			await new Promise((resolve, reject) => {
				let timeout;
				const interval = setInterval(() => {
					if (!!root.find({attributes: {role: %[1]q, name: %[2]q}})) {
						clearInterval(interval);
						clearTimeout(timeout)
						resolve();
					}
				}, 10);
				timeout = setTimeout(()=> {
					clearInterval(interval);
					reject('timed out waiting for node {role: %[1]q, name: %[2]q}');
				}, %[3]d);
			});
		})()`, role, name, int64(timeout/time.Millisecond))

	if err := f.tconn.EvalPromise(ctx, findQuery, nil); err != nil {
		f.logRoleDebugInfo(ctx, role)
		return err
	}
	return nil
}

// logRoleDebugInfo logs all elements with a role.
func (f *FilesApp) logRoleDebugInfo(ctx context.Context, role string) {
	var elements []string
	findQuery := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.findAll({attributes: {role: %q}}).map(node => node.name))", role)
	if err := f.tconn.EvalPromise(ctx, findQuery, &elements); err != nil {
		testing.ContextLogf(ctx, "Failed to grab debug info for {role: %s}: %s", role, err)
		return
	}
	testing.ContextLogf(ctx, "Debug info for %s: %+q", role, elements)
}
