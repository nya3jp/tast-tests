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

	"chromiumos/tast/errors"
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
	const (
		sourceApk     = "ArcDragSourceTest.apk"
		sourcePkg     = "org.chromium.arc.testapp.dragsource"
		sourceActName = ".DragSourceActivity"
		targetApk     = "ArcDragTargetTest.apk"
		targetPkg     = "org.chromium.arc.testapp.dragtarget"
		targetActName = ".DragTargetActivity"
	)

	startActivityWithBounds := func(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, apk, pkg, activityName string, wantBounds coords.Rect) (*arc.Activity, error) {
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			return nil, errors.Wrap(err, "failed installing app")
		}

		act, err := arc.NewActivity(a, pkg, activityName)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a new activity")
		}

		if err := act.Start(ctx, tconn); err != nil {
			act.Close()
			return nil, errors.Wrap(err, "failed to start the activity")
		}

		window, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
			return window.ARCPackageName == pkg
		})
		if err != nil {
			act.Stop(ctx, tconn)
			act.Close()
			return nil, errors.Wrap(err, "failed to find the ARC window")
		}

		if gotBounds, _, err := ash.SetWindowBounds(ctx, tconn, window.ID, wantBounds, window.DisplayID); err != nil {
			act.Stop(ctx, tconn)
			act.Close()
			return nil, errors.Wrap(err, "failed to set window bounds")
		} else if gotBounds != wantBounds {
			act.Stop(ctx, tconn)
			act.Close()
			return nil, errors.Errorf("failed to resize the activity: got %v; want %v", gotBounds, wantBounds)
		}
		return act, nil
	}

	assertDroppedDataInTargetActivity := func(ctx context.Context, d *ui.Device, expected string) error {
		const fieldID = targetPkg + ":id/dropped_data_view"
		if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expected, 30*time.Second); err != nil {
			return err
		}
		return nil
	}

	s.Run(ctx, "ChromeToAndroid", func(ctx context.Context, s *testing.State) {
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
		extID, err := chrome.ComputeExtensionID(extDir)
		if err != nil {
			s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
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

		s.Log("Connecting to extension background page")
		bgURL := chrome.ExtensionBackgroundPageURL(extID)
		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
		if err != nil {
			s.Fatalf("Could not connect to extension at %v: %v", bgURL, err)
		}
		defer conn.Close()

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

		wantBounds := coords.Rect{0, 0, 500, 500}

		act, err := startActivityWithBounds(ctx, a, tconn, targetApk, targetPkg, targetActName, wantBounds)
		if err != nil {
			s.Fatal("Failed to start an activity with bounds: ", err)
		}
		defer act.Close()
		defer act.Stop(ctx, tconn)

		srcPoint := coords.Point{X: 750, Y: 250}
		dstPoint := coords.Point{X: 250, Y: 250}
		if err := mouse.Drag(ctx, tconn, srcPoint, dstPoint, time.Second); err != nil {
			s.Fatal("Failed to send drag events: ", err)
		}

		if err := act.Focus(ctx, tconn); err != nil {
			s.Fatal("Failed to focus the activity: ", err)
		}

		const expected = `ClipData { text/plain "" {T:Data text} }`
		if err := assertDroppedDataInTargetActivity(ctx, d, expected); err != nil {
			s.Fatal("Assertion failed for dropped data in target activity: ", err)
		}
	})

	s.Run(ctx, "AndroidToAndroid", func(ctx context.Context, s *testing.State) {
		s.Log("Starting browser instance")
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
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

		sourceBounds := coords.Rect{0, 0, 500, 500}
		sourceAct, err := startActivityWithBounds(ctx, a, tconn, sourceApk, sourcePkg, sourceActName, sourceBounds)
		if err != nil {
			s.Fatal("Failed to start an activity with bounds: ", err)
		}
		defer sourceAct.Close()
		defer sourceAct.Stop(ctx, tconn)

		targetBounds := coords.Rect{500, 0, 500, 500}
		targetAct, err := startActivityWithBounds(ctx, a, tconn, targetApk, targetPkg, targetActName, targetBounds)
		if err != nil {
			s.Fatal("Failed to start an activity with bounds: ", err)
		}
		defer targetAct.Close()
		defer targetAct.Stop(ctx, tconn)

		if err := mouse.Drag(ctx, tconn, sourceBounds.CenterPoint(), targetBounds.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to send drag events: ", err)
		}

		if err := targetAct.Focus(ctx, tconn); err != nil {
			s.Fatal("Failed to focus the activity: ", err)
		}

		const expected = `ClipData { text/plain "" {T:hello world} }`
		if err := assertDroppedDataInTargetActivity(ctx, d, expected); err != nil {
			s.Fatal("Assertion failed for dropped data in target activity: ", err)
		}
	})
}
