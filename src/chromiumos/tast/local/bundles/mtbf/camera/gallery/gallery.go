// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gallery

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
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
	if err := WaitForElement(ctx, tconn, RoleButton, PlayButtonName, time.Minute); err != nil {
		return err
	}
	if err := ClickElement(ctx, tconn, RoleButton, PlayButtonName); err != nil {
		return err
	}
	return nil
}

// ClickElement clicks on the element with the specific role and name.
// If the JavaScript fails to execute, an error is returned.
func ClickElement(ctx context.Context, tconn *chrome.Conn, role string, name string) error {
	clickQuery := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.find({attributes: {role: %q, name: %q}}).doDefault());", role, name)
	if err := tconn.EvalPromise(ctx, clickQuery, nil); err != nil {
		return err
	}
	return nil
}

// WaitForElement waits for an element to exist.
func WaitForElement(ctx context.Context, tconn *chrome.Conn, role string, name string, timeout time.Duration) error {
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

	if err := tconn.EvalPromise(ctx, findQuery, nil); err != nil {
		return err
	}
	return nil
}
