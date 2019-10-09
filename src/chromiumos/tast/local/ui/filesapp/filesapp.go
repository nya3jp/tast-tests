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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/local/ui/apps"
)

// DownloadPath is the location of Downloads for the test user.
const DownloadPath = "/home/chronos/user/Downloads/"

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	tconn *chrome.Conn
}

// Launch launches the Files App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.Conn) (*FilesApp, error) {
	f := &FilesApp{tconn: tconn}
	// Launch the Files App.
	if err := apps.LaunchApp(ctx, tconn, apps.Files.ID); err != nil {
		return nil, err
	}
	// Wait for the Files App to be open.
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": "Files - My files", "role": "rootWebArea"},
	}
	if err := ui.WaitForNodeToAppear(ctx, tconn, params, time.Minute); err != nil {
		return nil, errors.Wrap(err, "failed to find element {role: 'Files - My files', name: 'rootWebArea'}")
	}
	return f, nil
}

// OpenDownloads opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads(ctx context.Context) error {
	// Find the Downloads label.
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": "Downloads", "role": "staticText"},
	}
	if err := ui.WaitForNodeToAppear(ctx, f.tconn, params, 10*time.Second); err != nil {
		return err
	}
	// Click Downloads to open the folder.
	if err := ui.LeftClick(ctx, f.tconn, params); err != nil {
		return err
	}

	// Ensure the Files App has switched to the Downloads folder.
	params = ui.FindParams{
		Attributes: map[string]interface{}{"name": "Files - Downloads", "role": "rootWebArea"},
	}
	return ui.WaitForNodeToAppear(ctx, f.tconn, params, 10*time.Second)
}

// SelectFile selects a file by clicking on it.
func (f *FilesApp) SelectFile(ctx context.Context, filename string) error {
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": filename, "role": "staticText"},
	}
	if err := ui.LeftClick(ctx, f.tconn, params); err != nil {
		return err
	}
	params.Attributes["name"] = fmt.Sprintf("Selected %s.", filename)
	return ui.WaitForNodeToAppear(ctx, f.tconn, params, 10*time.Second)
}

// WaitForFile waits for a file to be visible.
// An error is returned if the timeout is hit.
func (f *FilesApp) WaitForFile(ctx context.Context, filename string, timeout time.Duration) error {
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": filename, "role": "staticText"},
	}
	return ui.WaitForNodeToAppear(ctx, f.tconn, params, timeout)
}

// OpenQuickView opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(ctx context.Context, filename string) error {
	if err := f.SelectFile(ctx, filename); err != nil {
		return err
	}
	// TODO(bhansknecht@): Figure out why chrome.automation can't click items in the Files App menus.
	// Hopefully the keyboard can be removed from this
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()
	return kb.Accel(ctx, "Space")
}
