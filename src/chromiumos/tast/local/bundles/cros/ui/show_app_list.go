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

// Verifies that AppList's bounds in Peeking state are correct (see https://crbug.com/960174).
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

	// Enter tablet mode.
	if err := tconn.EvalPromise(ctx, `
	new Promise((resolve, reject) => {
		chrome.autotestPrivate.setTabletModeEnabled(true, isEnabled => {});
		resolve();
	})
	`, nil); err != nil {
		s.Fatal("Failed to enter tablet mode: ", err)
	}

	// Exit tablet mode.
	if err := tconn.EvalPromise(ctx, `
	new Promise((resolve, reject) => {
		chrome.autotestPrivate.setTabletModeEnabled(false, isEnabled => {});
		resolve();
	})
	`, nil); err != nil {
		s.Fatal("Failed to leave tablet mode: ", err)
	}

	// Click the Launcher button.
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

	var button_top int
	if err := tconn.EvalPromise(ctx, `
			new Promise((resolve, reject) => {
				chrome.automation.getDesktop(root => {
					const check = () => {
						const expand_arrow_view = root.find({ attributes: {name: 'Expand to all apps'}});
						if (expand_arrow_view) {
							resolve(expand_arrow_view.location['top']);
						}
						setTimeout(check, 10);
					}
					check();
				});
		})
		`, &button_top); err != nil {
		s.Fatal("Failed to find search box view: ", err)
	}

	var display_top int
	if err := tconn.EvalPromise(ctx, `
	    new Promise((resolve, reject) => {
				chrome.system.display.getInfo(null, info => {
					const check = () => {
						var l = info.length;
						for (var i = 0; i < l; i++) {
							if (info[i].isPrimary === true) {
								reject(info[i].workArea['top'] + info[i].workArea['height']);
							}
						}
						setTimeout(check, 10)
					}
          check();
			  });
	})
	`, &display_top); err != nil {
		s.Fatal("Failed to find display bounds: ", err)
	}
}
