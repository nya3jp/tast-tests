// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	mtbfui "chromiumos/tast/local/mtbf/ui"
	"chromiumos/tast/testing"
)

const (
	// VideoFolderPath video files download folder path
	VideoFolderPath = `/home/chronos/user/Downloads/videos/`
	// AudioFolderPath audio files download folder path
	AudioFolderPath = `/home/chronos/user/Downloads/audios/`
)

// MTBFFilesApp represent filesapp data model
type MTBFFilesApp struct {
	*filesapp.FilesApp
	tconn *chrome.TestConn
}

// Launch launches the Files App and returns it
func Launch(ctx context.Context, tconn *chrome.TestConn) (*MTBFFilesApp, error) {
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenFileApps, err)
	}

	return &MTBFFilesApp{
		FilesApp: files,
		tconn:    tconn,
	}, nil
}

// WaitForElement waits for an element to exist.
// If the timeout is reached, an error is returned.
func (f *MTBFFilesApp) WaitForElement(ctx context.Context, role ui.RoleType, name string, timeout time.Duration) error {
	params := ui.FindParams{
		Name: name,
		Role: role,
	}
	if err := ui.WaitUntilExists(ctx, f.tconn, params, timeout); err != nil {
		return err
	}
	return nil
}

var scriptTemplate = map[string]string{
	"getFilesApp": `const getFilesApp =
		(root, folder) =>
			root.find({ attributes: { role: 'rootWebArea', name: 'Files - ' + folder } });`,
	"getMainListBox": `const getMainListBox =
		root => root
			.find({ attributes: { role: 'main' } })
			.find({ attributes: { role: 'listBox' } });`,
}

// FocusOnFilesApp make window focus FilesApp
func FocusOnFilesApp(ctx context.Context, conn *chrome.TestConn, files *MTBFFilesApp, currentFolder string) (bool, *mtbferrors.MTBFError) {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) =>
			chrome.automation.getFocus(node => {
				%v
				const { name, className, role } = node;
				const nodeDesc = { name, className, role };
				chrome.automation.getDesktop(root => {
					const focusNode = getFilesApp(root, %q).find(nodeDesc);
					console.log({ focusNode });
					resolve(!!focusNode);
				});
			})
		);`, scriptTemplate["getFilesApp"], currentFolder)

	isFocus := true
	if err := conn.EvalPromise(ctx, script, &isFocus); err != nil {
		return isFocus, mtbferrors.New(mtbferrors.ChromeExeJs, err, "FocusOnFilesApp")
	}

	if !isFocus {
		if err := mtbfui.ClickElement(ctx, conn, ui.RoleTypeButton, "Files"); err != nil {
			return isFocus, mtbferrors.New(mtbferrors.ChromeClickItem, err, "Files")
		}
		testing.Sleep(ctx, 2*time.Second) // For possible UI update delay
	}

	return isFocus, nil
}

// LogFilesUnderCurrentFolder logs all files under a folder.
func LogFilesUnderCurrentFolder(ctx context.Context, conn *chrome.TestConn) (string, *mtbferrors.MTBFError) {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				%v
				const mainListBox = getMainListBox(root);
				if (mainListBox) {
					const listBoxOptions = mainListBox.findAll({ attributes: { role: 'listBoxOption' } });
					if (listBoxOptions && listBoxOptions.length) {
						resolve(listBoxOptions.map(node => node.name).join('\n'));
					} else {
						resolve('No files available.');
					}
				} else {
					reject('Can\'t find target: role="main" > role="listBox".')
				}
			});
		});`, scriptTemplate["getMainListBox"])

	logs := ""
	if err := conn.EvalPromise(ctx, script, &logs); err != nil {
		return logs, mtbferrors.New(mtbferrors.ChromeExeJs, err, "LogFilesUnderCurrentFolder")
	}

	return logs, nil
}

// SortFilesByModifiedDateInDescendingOrder sorts all files in File app with descending order
func (f *MTBFFilesApp) SortFilesByModifiedDateInDescendingOrder(ctx context.Context) error {
	testing.Sleep(ctx, 2*time.Second)

	params := ui.FindParams{
		Name: "Name",
		Role: ui.RoleTypeButton,
	}
	nameSort, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return err
	}
	defer nameSort.Release(ctx)
	if err := nameSort.LeftClick(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, 2*time.Second)

	params = ui.FindParams{
		Name: "Date modified",
		Role: ui.RoleTypeButton,
	}
	dateSort, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return err
	}
	defer dateSort.Release(ctx)
	if err := dateSort.LeftClick(ctx); err != nil {
		return err
	}

	return nil
}

// WaitAndClickElement wait element appear and click
func (f *MTBFFilesApp) WaitAndClickElement(ctx context.Context, role ui.RoleType, name string) *mtbferrors.MTBFError {
	if err := f.WaitForElement(ctx, role, name, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, name)
	}
	testing.Sleep(ctx, 1*time.Second) // For possible UI update delay
	if err := mtbfui.ClickElement(ctx, f.tconn, role, name); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, name)
	}
	return nil
}

// WaitAndEnterElement wait for specified dom element rendered and send "Enter" event
func (f *MTBFFilesApp) WaitAndEnterElement(ctx context.Context, role ui.RoleType, name string) *mtbferrors.MTBFError {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()

	if err := f.WaitForElement(ctx, role, name, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, name)
	}
	if err := mtbfui.ClickElement(ctx, f.tconn, role, name); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, name)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	return nil
}

// EnterFolderPath enter nested folder in File app
func (f *MTBFFilesApp) EnterFolderPath(ctx context.Context, folders []string) *mtbferrors.MTBFError {
	for _, folder := range folders {

		if err := f.WaitAndEnterElement(ctx, ui.RoleTypeStaticText, folder); err != nil {
			return mtbferrors.New(mtbferrors.ChromeClickItem, err, folder)
		}
		testing.Sleep(ctx, 2*time.Second) // Wait for ui to update
	}

	return nil
}
