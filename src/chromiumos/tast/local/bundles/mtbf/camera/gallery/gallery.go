// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gallery

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	mtbfui "chromiumos/tast/local/mtbf/ui"
)

const (
	// GalleryID is AppID for connection.
	GalleryID = "nlkncpkkdoccmpiclbokaimcnedabhhm"

	// RoleButton is the chrome.automation role for buttons.
	RoleButton = "button"

	// PlayButtonName is the expected play button name.
	PlayButtonName = "play"
)

// Close closes Gallery.
func Close(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	if err := apps.Close(ctx, tconn, GalleryID); err != nil {
		return err
	}
	return nil
}

// PlayVideo plays video from Gallery.
func PlayVideo(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	if err := mtbfui.WaitForElement(ctx, tconn, RoleButton, PlayButtonName, time.Minute); err != nil {
		return err
	}
	if err := mtbfui.ClickElement(ctx, tconn, RoleButton, PlayButtonName); err != nil {
		return err
	}
	return nil
}
