// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOmnibox,
		Desc:         "Checks that the virtual keyboard appears when clicking on the omnibox",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardOmnibox(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Start a empty window.
	if _, err := cr.NewConn(ctx, "chrome://newtab"); err != nil {
		s.Fatal("Failed to start a new tab: ", err)
	}

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

	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get UI automation root: ", err)
	}
	defer root.Release(ctx)

	// Click on the omnibox.
	params := ui.FindParams{
		Role:       ui.RoleTypeTextField,
		Attributes: map[string]interface{}{"inputType": "url"},
	}
	omnibox, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for the omnibox: ", err)
	}
	defer omnibox.Release(ctx)

	if err := omnibox.LeftClick(ctx); err != nil {
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
