// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragDrop,
		Desc:         "Checks drag and drop support from Chrome to ARC",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"drag_drop_manifest.json", "drag_drop_background.js", "drag_drop_window.js", "drag_drop_window.html"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func DragDrop(ctx context.Context, s *testing.State) {
	s.Log("Copying extension to temp directory")
	extDir, err := ioutil.TempDir("", "tast.arc.DragDropExtension")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)
	for _, name := range []string{"manifest.json", "background.js", "window.js", "window.html"} {
		if err := fsutil.CopyFile(s.DataPath("drag_drop_"+name), filepath.Join(extDir, name)); err != nil {
			s.Fatalf("Failed to copy extension %s: %v", name, err)
		}
	}

	s.Log("Starting browser instance")
	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir), chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	const (
		apk          = "ArcDragDropTest.apk"
		pkg          = "org.chromium.arc.testapp.dragdrop"
		activityName = "org.chromium.arc.testapp.dragdrop.DragDropActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	window, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return window.ARCPackageName == pkg
	})
	if err != nil {
		s.Fatal("Failed to find the ARC window: ", err)
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(cur *ash.Window) bool {
		return cur.ID == window.ID && cur.State == ash.WindowStateNormal && !cur.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the window to finish animating: ", err)
	}

	wantBounds := coords.Rect{Left: 0, Top: 0, Width: 500, Height: 500}

	if gotBounds, _, err := ash.SetWindowBounds(ctx, tconn, window.ID, wantBounds, window.DisplayID); err != nil {
		s.Fatal("Failed to set window bounds: ", err)
	} else if gotBounds != wantBounds {
		s.Fatalf("Failed to resize the activity: got %v; want %v", gotBounds, wantBounds)
	}

	srcPoint := coords.Point{X: 750, Y: 250}
	dstPoint := coords.Point{X: 250, Y: 250}
	if err := mouse.Drag(ctx, tconn, srcPoint, dstPoint, time.Second); err != nil {
		s.Fatal("Failed to send drag events: ", err)
	}

	if err := act.Focus(ctx, tconn); err != nil {
		s.Fatal("Failed to focus the activity: ", err)
	}

	const (
		fieldID  = pkg + ":id/dropped_data_view"
		expected = `ClipData { text/plain "" {T:Data text} }`
	)

	if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expected, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for the drag and drop result: ", expected)
	}
}
