// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
)

// DriveWeb represents recent files from Google Drive web
type DriveWeb struct {
	cr           *chrome.Chrome
	tconn        *chrome.TestConn
	recentFiles  []string
	trashedFiles []string
}

const (
	// Recent URL for Drive Web
	Recent = "https://drive.google.com/drive/recent"
	// Trash URL for Drive Web
	Trash = "https://drive.google.com/drive/trash"
)

// Setup gets the Recent files from Drive Web and returns an instance of DriveWeb.
func Setup(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*DriveWeb, error) {
	recentFiles, err := waitForFileThenGetFilesList(ctx, cr, Recent, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed getting drive web recent files")
	}

	trashedFiles, err := waitForFileThenGetFilesList(ctx, cr, Trash, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed getting drive web trashed files")
	}

	return &DriveWeb{cr, tconn, recentFiles, trashedFiles}, nil
}

// WaitForRecentFile waits for the supplied file name to appear in the files since last cache.
func (d *DriveWeb) WaitForRecentFile(ctx context.Context, name string) error {
	mostRecentFiles, err := waitForFileThenGetFilesList(ctx, d.cr, Recent, name)
	if err != nil {
		return errors.Wrap(err, "failed getting drive recent files")
	}

	if err := hasFileRecentlyChanged(ctx, name, d.recentFiles, mostRecentFiles); err != nil {
		return err
	}

	return nil
}

// WaitForDeletedFile waits for the file name to appear in the Trash files diff since last cache.
func (d *DriveWeb) WaitForDeletedFile(ctx context.Context, name string) error {
	mostRecentTrashedFiles, err := waitForFileThenGetFilesList(ctx, d.cr, Trash, name)
	if err != nil {
		return errors.Wrap(err, "failed getting drive trashed files")
	}

	if err := hasFileRecentlyChanged(ctx, name, d.trashedFiles, mostRecentTrashedFiles); err != nil {
		return err
	}

	return nil
}

// MinimizeWindow sets the window state to minimized by sending an ash event.
func (d *DriveWeb) MinimizeWindow(ctx context.Context) error {
	// Minimize the Play Store window to allow access to Install.
	window, err := ash.FindWindow(ctx, d.tconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "Google Drive")
	})
	if err != nil {
		return errors.Wrap(err, "failed to find the Google Drive window")
	}
	if _, err := ash.SetWindowState(ctx, d.tconn, window.ID, ash.WMEventMinimize); err != nil {
		return errors.Wrap(err, "failed to minimize Google Drive window")
	}

	return nil
}

// hasFileRecentlyChanged checks the most recently changed files (trashed or recent) and looks for the name
func hasFileRecentlyChanged(ctx context.Context, name string, originalFiles, newFiles []string) error {
	for _, identifiedName := range newFiles {
		if identifiedName != originalFiles[0] {
			if identifiedName == name {
				return nil
			}
		} else {
			break
		}
	}

	return errors.Errorf("failed to identify %q in files recently changed", name)
}

// waitForFileThenGetFilesList waits until a file appears in Recent files, then returns the list.
func waitForFileThenGetFilesList(ctx context.Context, cr *chrome.Chrome, url, name string) ([]string, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed setting up a connection")
	}
	defer conn.Close()

	if name != "" {
		jsExpr := fmt.Sprintf(`Array.from(document.querySelectorAll('div[role="option"]')).filter(x => x.innerText.indexOf(%q) >= 0).length > 0`, name)
		if err := conn.WaitForExprFailOnErrWithTimeout(ctx, jsExpr, time.Minute); err != nil {
			return nil, errors.Wrapf(err, "failed waiting for file %q at url %q", name, url)
		}
	}

	jsExpr := `Array.from(document.querySelectorAll('div[role="option"]')).map(x => x.innerText)`
	var files []string
	err = conn.Eval(ctx, jsExpr, &files)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting list of files using js at url %q", url)
	}

	return files, nil
}
