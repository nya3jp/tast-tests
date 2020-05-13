// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeDisplay,
		Desc:         "Demonstrates how to use the chrome.display API",
		Contacts:     []string{"ricardoq@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChromeDisplay(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("No display: ", err)
	}
	s.Logf("Display info: %+v", info)
	s.Log("Supported display modes:")
	for _, m := range info.Modes {
		s.Logf("Mode: %+v", *m)
	}

	newZoom := info.AvailableDisplayZoomFactors[0]
	newRot := 90

	s.Logf("Setting display properties: rotation=%d, zoom=%v", newRot, newZoom)
	p := display.DisplayProperties{DisplayZoomFactor: &newZoom, Rotation: &newRot}
	if err := display.SetDisplayProperties(ctx, tconn, info.ID, p); err != nil {
		s.Fatal("Failed to set display properties: ", err)
	}

	// Leave Chrome in a reasonable state. Restore zoom and rotation.
	s.Logf("Restoring display properties: rotation=%d, zoom=%v", info.Rotation, info.DisplayZoomFactor)
	p = display.DisplayProperties{DisplayZoomFactor: &info.DisplayZoomFactor, Rotation: &info.Rotation}
	if err := display.SetDisplayProperties(ctx, tconn, info.ID, p); err != nil {
		s.Fatal("Failed to set display properties: ", err)
	}
}
