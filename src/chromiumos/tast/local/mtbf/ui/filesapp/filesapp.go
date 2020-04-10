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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

// Launch launches the Files App and returns it.
// A mtbf error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.Conn) (*filesapp.FilesApp, error) {
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return files, mtbferrors.New(mtbferrors.ChromeOpenFileApps, err)
	}
	return files, err
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
func FocusOnFilesApp(ctx context.Context, conn *chrome.Conn, files *filesapp.FilesApp, currentFolder string) (bool, *mtbferrors.MTBFError) {
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
		if err := files.ClickElement(ctx, filesapp.RoleButton, "Files"); err != nil {
			return isFocus, mtbferrors.New(mtbferrors.ChromeClickItem, err, "Files")
		}
		testing.Sleep(ctx, 2*time.Second) // For possible UI update delay
	}

	return isFocus, nil
}

// LogFilesUnderCurrentFolder logs all files under a folder.
func LogFilesUnderCurrentFolder(ctx context.Context, conn *chrome.Conn) (string, *mtbferrors.MTBFError) {
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

// SortFilesByModifiedDateInDescendingOrder sort files under current folder by modified date in descending order
func SortFilesByModifiedDateInDescendingOrder(ctx context.Context, files *filesapp.FilesApp) *mtbferrors.MTBFError {
	testing.Sleep(ctx, 2*time.Second) // For possible UI update delay
	if err := files.ClickElement(ctx, filesapp.RoleButton, "Name"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, "button 'Name'")
	}
	testing.Sleep(ctx, 2*time.Second) // For possible UI update delay
	if err := files.ClickElement(ctx, filesapp.RoleButton, "Date modified"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, "button 'Date modified'")
	}
	testing.Sleep(ctx, 2*time.Second) // For possible UI update delay
	return nil
}

// WaitAndClickElement wait for specified dom element rendered and click
func WaitAndClickElement(ctx context.Context, f *filesapp.FilesApp, role string, name string) *mtbferrors.MTBFError {
	if err := f.WaitForElement(ctx, role, name, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, name)
	}
	testing.Sleep(ctx, 1*time.Second) // For possible UI update delay
	if err := f.ClickElement(ctx, role, name); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, name)
	}
	return nil
}

// WaitAndEnterElement wait for specified dom element rendered and send "Enter" event
func WaitAndEnterElement(ctx context.Context, f *filesapp.FilesApp, role string, name string) *mtbferrors.MTBFError {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()

	if err := f.WaitForElement(ctx, role, name, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, name)
	}
	if err := f.ClickElement(ctx, role, name); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, name)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	return nil
}
