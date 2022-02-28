// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

type dragDropTestArgs struct {
	extensionPrefix string
	androidSource   bool
	androidTarget   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragDrop,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks drag and drop support from Chrome to ARC",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			"drag_drop_manifest.json", "drag_source_background.js", "drag_source_window.js", "drag_source_window.html",
			"drag_target_background.js", "drag_target_window.js", "drag_target_window.html"},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name:              "chrome_to_android",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: &dragDropTestArgs{
				extensionPrefix: "drag_source_",
				androidSource:   false,
				androidTarget:   true,
			},
		}, {
			Name:              "chrome_to_android_vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: &dragDropTestArgs{
				extensionPrefix: "drag_source_",
				androidSource:   false,
				androidTarget:   true,
			},
		}, {
			Name:              "android_to_android",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			Val: &dragDropTestArgs{
				androidSource: true,
				androidTarget: true,
			},
		}, {
			Name:              "android_to_android_vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: &dragDropTestArgs{
				androidSource: true,
				androidTarget: true,
			},
		}, {
			Name:              "android_to_chrome",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			Val: &dragDropTestArgs{
				extensionPrefix: "drag_target_",
				androidSource:   true,
				androidTarget:   false,
			},
		}, {
			Name:              "android_to_chrome_vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: &dragDropTestArgs{
				extensionPrefix: "drag_target_",
				androidSource:   true,
				androidTarget:   false,
			},
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

		// width and height of target and source windows.
		w = 500
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	args := s.Param().(*dragDropTestArgs)

	var extID string
	chromeOpts := []chrome.Option{chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"), chrome.ExtraArgs("--disable-features=ArcResizeLock")}
	if args.extensionPrefix != "" {
		s.Log("Copying extension to temp directory")
		extDir, err := ioutil.TempDir("", "tast.arc.DragDropExtension")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		defer os.RemoveAll(extDir)
		if err := fsutil.CopyFile(s.DataPath("drag_drop_manifest.json"), filepath.Join(extDir, "manifest.json")); err != nil {
			s.Fatal("Failed to copy extension manifest.json: ", err)
		}
		for _, name := range []string{"background.js", "window.js", "window.html"} {
			if err := fsutil.CopyFile(s.DataPath(args.extensionPrefix+name), filepath.Join(extDir, name)); err != nil {
				s.Fatalf("Failed to copy extension %s: %v", name, err)
			}
		}
		extID, err = chrome.ComputeExtensionID(extDir)
		if err != nil {
			s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
		}
		chromeOpts = append(chromeOpts, chrome.UnpackedExtension(extDir))
	}

	s.Log("Starting browser instance")
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	startActivityWithBounds := func(ctx context.Context, apk, pkg, activityName string, wantBounds coords.Rect) (act *arc.Activity, err error) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
		defer cancel()

		if err = a.Install(ctx, arc.APKPath(apk)); err != nil {
			err = errors.Wrap(err, "failed installing app")
			return
		}

		if act, err = arc.NewActivity(a, pkg, activityName); err != nil {
			err = errors.Wrap(err, "failed to create a new activity")
			return
		}

		if err = act.StartWithDefaultOptions(ctx, tconn); err != nil {
			act.Close()
			act = nil
			err = errors.Wrap(err, "failed to start the activity")
			return
		}

		defer func() {
			if err != nil {
				act.Stop(cleanupCtx, tconn)
				act.Close()
				act = nil
			}
		}()

		var window *ash.Window
		if window, err = ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
			return window.ARCPackageName == pkg
		}); err != nil {
			err = errors.Wrap(err, "failed to find the ARC window")
			return
		}

		if err = act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
			err = errors.Wrap(err, "failed to set the window state to normal")
			return
		}

		if err = ash.WaitForCondition(ctx, tconn, func(cur *ash.Window) bool {
			return cur.ID == window.ID && cur.State == ash.WindowStateNormal && !cur.IsAnimating
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			err = errors.Wrap(err, "failed to wait for the window to finish animating")
			return
		}

		var gotBounds coords.Rect
		if gotBounds, _, err = ash.SetWindowBounds(ctx, tconn, window.ID, wantBounds, window.DisplayID); err != nil {
			err = errors.Wrap(err, "failed to set window bounds")
			return
		} else if gotBounds != wantBounds {
			err = errors.Errorf("failed to resize the activity: got %v; want %v", gotBounds, wantBounds)
			return
		}
		return
	}

	sourceBounds := coords.Rect{Left: w, Top: 0, Width: w, Height: w}
	targetBounds := coords.Rect{Left: 0, Top: 0, Width: w, Height: w}

	if args.androidSource {
		sourceAct, err := startActivityWithBounds(ctx, sourceApk, sourcePkg, sourceActName, sourceBounds)
		if err != nil {
			s.Fatal("Failed to start an activity with bounds: ", err)
		}
		defer sourceAct.Close()
		defer sourceAct.Stop(cleanupCtx, tconn)
	}

	var targetAct *arc.Activity
	if args.androidTarget {
		targetAct, err = startActivityWithBounds(ctx, targetApk, targetPkg, targetActName, targetBounds)
		if err != nil {
			s.Fatal("Failed to start an activity with bounds: ", err)
		}
		defer targetAct.Close()
		defer targetAct.Stop(cleanupCtx, tconn)
	}

	if err := mouse.Drag(tconn, sourceBounds.CenterPoint(), targetBounds.CenterPoint(), time.Second)(ctx); err != nil {
		s.Fatal("Failed to send drag events: ", err)
	}

	if args.androidTarget {
		if err := targetAct.Focus(ctx, tconn); err != nil {
			s.Fatal("Failed to focus the activity: ", err)
		}

		const (
			expected = `ClipData { text/plain "" {T:Data text} }`
			fieldID  = targetPkg + ":id/dropped_data_view"
		)
		if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expected, 30*time.Second); err != nil {
			s.Fatal("Failed to wait for the drag and drop result: ", err)
		}
	} else {
		s.Log("Connecting to the extension page")
		bgURL := "chrome-extension://" + extID + "/window.html"
		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
		if err != nil {
			s.Fatalf("Could not connect to extension at %v: %v", bgURL, err)
		}

		const expected = "Data text"
		if err := conn.WaitForExpr(ctx, fmt.Sprintf(`document.getElementById('dropped-data').innerHTML === %q`, expected)); err != nil {
			s.Fatal("Failed to wait for the dropped data: ", err)
		}
	}
}
