// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filepicker supports controlling the file picker on ChromeOS.
package filepicker

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filepicker/vars"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
)

// FilePicker represents an instance of the file picker.
type FilePicker struct {
	filesApp *filesapp.FilesApp
}

// Find returns an existing instance of the File picker.
// An error is returned if the picker cannot be found.
func Find(ctx context.Context, tconn *chrome.TestConn) (*FilePicker, error) {
	filesApp, err := filesapp.App(ctx, tconn, vars.FilePickerPseudoAppID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find file picker")
	}
	return &FilePicker{filesApp: filesApp}, nil
}

// OpenDir returns a function that opens one of the directories shown in the navigation tree.
// An error is returned if dir is not found or does not open.
func (f *FilePicker) OpenDir(dirName string) uiauto.Action {
	return f.filesApp.OpenDir(dirName, dirName)
}

// OpenFile returns a function that executes double click on a file to open it with default app.
func (f *FilePicker) OpenFile(fileName string) uiauto.Action {
	return f.filesApp.OpenFile(fileName)
}
