// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package linuxfiles supports actions on Linux files on Files app.
package linuxfiles

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
)

// Linux files directory name and title in Files app.
const (
	DirName = "Linux files"
	Title   = "Files - " + DirName
)

// Folder sharing strings.
const (
	ManageLinuxSharing = "Manage Linux Sharing"
	DialogName         = "Share folder with Linux"
	MountPath          = "/mnt/chromeos"
	MountFolderMyFiles = "MyFiles"
	MountPathMyFiles   = MountPath + "/" + MountFolderMyFiles
)

// Strings for sharing My files.
const (
	MyFilesMsg = "Give Linux apps permission to modify files in the My files folder"
	MyFiles    = "My files"
)

const uiTimeout = 15 * time.Second

type shareConfirmDialog struct {
	okButton     *ui.Node
	cancelButton *ui.Node
}

type shareToastNotification struct {
	manageButton *ui.Node
}

func findShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string) (scd *shareConfirmDialog, err error) {
	// Find the dialog, confirmation message, Cancel and OK button.
	dialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: DialogName, Role: ui.RoleTypeDialog}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation dialog")
	}
	defer dialog.Release(ctx)

	if _, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: msg, Role: ui.RoleTypeStaticText}, uiTimeout); err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation message")
	}

	ok, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button OK in confimration dialog")
	}

	cancel, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "Cancel", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button Cancel in confimration dialog")
	}

	return &shareConfirmDialog{okButton: ok, cancelButton: cancel}, nil
}

func (scd *shareConfirmDialog) clickOK(ctx context.Context) error {
	defer scd.okButton.Release(ctx)

	if err := scd.okButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK")
	}
	return nil
}

func (scd *shareConfirmDialog) clickCancel(ctx context.Context) error {
	defer scd.cancelButton.Release(ctx)

	if err := scd.cancelButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Cancel")
	}
	return nil
}

func findShareToastNotification(ctx context.Context, tconn *chrome.TestConn) (toast *shareToastNotification, err error) {
	// Find the toast notification, message and button Manage.
	toastNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "container", Role: ui.RoleTypeAlert}, uiTimeout)
	if err != nil {
		return toast, errors.Wrap(err, "failed to find toast nofication")
	}
	defer toastNode.Release(ctx)

	if _, err := toastNode.DescendantWithTimeout(ctx, ui.FindParams{Name: "1 folder shared with Linux", Role: ui.RoleTypeStaticText}, uiTimeout); err != nil {
		return toast, errors.Wrap(err, "failed to find toast message")
	}

	manage, err := toastNode.DescendantWithTimeout(ctx, ui.FindParams{Name: "Manage", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return toast, errors.Wrap(err, "failed to find button Manage in toast notification")
	}
	return &shareToastNotification{manageButton: manage}, nil
}

func (toast *shareToastNotification) clickManage(ctx context.Context, tconn *chrome.TestConn) error {
	defer toast.manageButton.Release(ctx)

	// Move the mouse here because the toast flies from the bottom of Files app to a bit above.
	if err := mouse.Move(ctx, tconn, toast.manageButton.Location.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mounse to button Manage")
	}
	if err := toast.manageButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button Manage")
	}

	return nil
}

// CheckShareConfirmDialog checks the content in the folder sharing confirmation dialog.
// OK is clicked if clickOK is true, otherwise Cancel is clicked.
func CheckShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string, clickOK bool) error {
	scd, err := findShareConfirmDialog(ctx, tconn, msg)
	if err != nil {
		return errors.Wrap(err, "failed to check share confirmation dialog")
	}

	// Click button cancel if clickOK is false.
	if clickOK {
		// Click button OK.
		if err := scd.clickOK(ctx); err != nil {
			return errors.Wrap(err, "failed to click button OK")
		}
	} else {
		if err := scd.clickCancel(ctx); err != nil {
			return errors.Wrap(err, "failed to click button Cancel")
		}

		// Make sure that there is no sharing toast notification.
		if _, err := findShareToastNotification(ctx, tconn); err == nil {
			return errors.New("toast notification is displayed unexpectedly after clicking button Cancel")
		}
	}

	return nil
}

// CheckShareToast checks the content of the sharing toast and clicks Manage if clickManage is true.
func CheckShareToast(ctx context.Context, tconn *chrome.TestConn, clickManage bool) error {
	// Make sure that there is no sharing toast notification.
	toast, err := findShareToastNotification(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to check toast notification")
	}

	if clickManage {
		if err := toast.clickManage(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to click button Manage")
		}
	}

	return nil
}
