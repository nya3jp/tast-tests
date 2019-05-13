// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowAppList,
		Desc:         "Checks that the AppList has correct bounds after pessing the AppList button",
		Contacts:     []string{"andrewxu@chromium.org", "newcomer@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ShowAppList(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := tconn.EvalPromise(ctx, `
	new Promise((resolve, reject) => {
		chrome.autotestPrivate.setTabletModeEnabled(true, isEnabled => {});
	})
	`, nil); err != nil {
		s.Fatal("Failed to enter tablet mode: ", err)
	}

	if err := tconn.EvalPromise(ctx, `
	new Promise((resolve, reject) => {
		chrome.automation.getDesktop(root => {
			const check = () => {
					const shelf_button_view = root.find({ attributes: {role: 'button', name: 'Launcher'}});
          if (shelf_button_view) {
						shelf_button_view.doDefault();
						resolve();
						return;
					}
					setTimeout(check, 10);
				}
				check();
			});
	})
	`, nil); err != nil {
		s.Fatal("Failed to focus on the launcher button: ", err)
	}
}
