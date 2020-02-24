// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartSelectionChrome,
		Desc:         "Test ARC's smart selections show up in Chrome's right click menu",
		Contacts:     []string{"bhansknecht@chromium.org", "dhaddock@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"arc.SmartSelectionChrome.username", "arc.SmartSelectionChrome.password"},
	})
}

func SmartSelectionChrome(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.SmartSelectionChrome.username")
	password := s.RequiredVar("arc.SmartSelectionChrome.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close()
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Open page with an address on it.
	if _, err := cr.NewConn(ctx, "https://google.com/search?q=1600+amphitheatre+parkway"); err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}

	// Wait for the address to appear.
	const node = "{role: 'staticText', name: '1600 amphitheatre parkway'}"
	findQuery := fmt.Sprintf(
		`(async () => {
		  const root = await tast.promisify(chrome.automation.getDesktop)();
		  await new Promise((resolve, reject) => {
		    let timeout;
		    const interval = setInterval(() => {
		      if (!!root.find({attributes: %[1]s})) {
		        clearInterval(interval);
		        clearTimeout(timeout);
		        resolve();
		      }
		    }, 10);
		    timeout = setTimeout(()=> {
		      clearInterval(interval);
		      reject("timed out waiting for node %[1]s");
		    }, 10000);
		  });
		})()`, node)
	if err := tconn.EvalPromise(ctx, findQuery, nil); err != nil {
		s.Fatal("Failed to wait for address to load: ", err)
	}

	// Select the address.
	selectQuery := fmt.Sprintf(
		`tast.promisify(chrome.automation.getDesktop)().then(
		  root => root.find({attributes: %s})
		).then(
		  x => chrome.automation.setDocumentSelection({anchorObject: x, anchorOffset: 0, focusObject: x, focusOffset: 25})
		);`, node)
	if err := tconn.EvalPromise(ctx, selectQuery, nil); err != nil {
		s.Fatal("Failed to select address: ", err)
	}

	// Right click the selected address.
	rightClickQuery := fmt.Sprintf(
		`tast.promisify(chrome.automation.getDesktop)().then(
		  root => root.find({attributes: %s}).showContextMenu()
		);`, node)
	if err := tconn.EvalPromise(ctx, rightClickQuery, nil); err != nil {
		s.Fatal("Failed to right click address: ", err)
	}

	// Ensure the smart selection map option is available.
	findQuery =
		`(async () => {
		  const root = await tast.promisify(chrome.automation.getDesktop)();
		  await new Promise((resolve, reject) => {
		    let timeout;
		    const interval = setInterval(() => {
		      if (!!root.find({attributes: {role: "menuItem", name: "Map"}})) {
		        clearInterval(interval);
		        clearTimeout(timeout);
		        resolve();
		      }
		    }, 10);
		    timeout = setTimeout(()=> {
		      clearInterval(interval);
		      reject('timed out waiting for node {role: "menuItem", name: "Map"}');
		    }, 10000);
		  });
		})()`
	if err := tconn.EvalPromise(ctx, findQuery, nil); err != nil {
		s.Fatal("Failed to show map option: ", err)
	}
}
