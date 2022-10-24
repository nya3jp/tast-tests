// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filepicker supports controlling the file picker on ChromeOS.
package filepicker

import (
	"context"
	"fmt"
	"time"

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

// OpenFile returns a function that executes double click on a file to open it.
func (f *FilePicker) OpenFile(fileName string) uiauto.Action {
	// For the file picker, opening the file should close the picker.
	// We retry opening the file three times to deflake some tests,
	// as sometimes the double-click seems to be ignored.
	return uiauto.Retry(3,
		uiauto.Combine(fmt.Sprintf("OpenFile(%s)", fileName),
			f.filesApp.SelectFile(fileName),
			f.filesApp.OpenFile(fileName),
			f.filesApp.WithTimeout(3*time.Second).WaitUntilGone(filesapp.WindowFinder(vars.FilePickerPseudoAppID)),
		))
}

// SelectFile returns a function that selects a file.
func (f *FilePicker) SelectFile(fileName string) uiauto.Action {
	return f.filesApp.SelectFile(fileName)
}

// WithTimeout returns a new FilePicker with the specified timeout.
// This only changes the timeout and does not relaunch the FilePicker.
func (f *FilePicker) WithTimeout(timeout time.Duration) *FilePicker {
	return &FilePicker{filesApp: f.filesApp.WithTimeout(timeout)}
}
