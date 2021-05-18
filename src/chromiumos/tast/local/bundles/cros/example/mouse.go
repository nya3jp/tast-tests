// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Mouse,
		Desc:         "Demonstrates how to use the chrome.autotestPrivate.mouseMove with multiple displays",
		Contacts:     []string{"mukai@chromium.org"},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func Mouse(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}

	s.Log("Waiting for 5 seconds to ensure that the displays are fully turned on")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) < 2 {
		s.Log("No external displays exist, not conducting MoveInDisplay")
		return
	}

	var internal, external display.Info
	for i, info := range infos {
		s.Logf("Display %d: %+v", i, info)
		if info.IsInternal {
			internal = info
		} else {
			external = info
		}
	}

	if err := action.Combine(
		"mouse drag",
		mouse.Move(tconn, external.Bounds.CenterPoint(), 0),
		mouse.Move(tconn, internal.Bounds.CenterPoint(), 5*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to move the mouse: ", err)
	}
}
