// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOmnibox,
		Desc:         "Checks that the virtual keyboard appears when clicking on the omnibox",
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome_login", "virtual_keyboard"},
	})
}

func VirtualKeyboardOmnibox(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	shown, err := vkb.IsShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if the virtual keyboard is initially hidden: ", err)
	}
	if shown {
		s.Fatal("Virtual keyboard is shown, but expected it to be hidden")
	}

	// Click on the omnibox.
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const check = () => {
			const omnibox = root.find({ attributes: { role: 'textField', inputType: 'url' }});
			if (omnibox) {
				omnibox.doDefault();
				resolve();
				return;
			}
			setTimeout(check, 10);
		}
		check();
	});
})
`, nil); err != nil {
		s.Fatal("Failed to click the omnibox: ", err)
	}

	// Record the time it takes to render the virtual keyboard.
	start := time.Now()

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	elapsed := time.Since(start)

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "virtual_keyboard_initial_load_time",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(elapsed/time.Millisecond))

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
