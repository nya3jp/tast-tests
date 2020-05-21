// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragDrop,
		Desc:         "Checks drag and drop support from Chrome to ARC",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"drag_drop_manifest.json", "drag_drop_background.js", "drag_drop_window.js", "drag_drop_window.html"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               []string{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               []string{"--enable-arcvm"},
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
	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	s.Log("Starting browser instance")
	args := s.Param().([]string)
	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir), chrome.ARCEnabled(), chrome.ExtraArgs(args...))
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

	var deviceScaleRatio json.Number
	if err := conn.Eval(ctx, "window.devicePixelRatio", &deviceScaleRatio); err != nil {
		s.Fatal("window.devicePixelRatio API unavailable: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const (
		apk             = "ArcDragDropTest.apk"
		pkg             = "org.chromium.arc.testapp.dragdrop"
		startupActivity = "org.chromium.arc.testapp.dragdrop.StartupActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, startupActivity)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-n"}, []string{"--ef", "DEVICE_SCALE_FACTOR", deviceScaleRatio.String()}); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx)

	srcPoint := coords.Point{X: 450, Y: 150}
	dstPoint := coords.Point{X: 150, Y: 150}
	if err := mouse.Drag(ctx, tconn, srcPoint, dstPoint, time.Second); err != nil {
		s.Fatal("Failed to send drag events: ", err)
	}

	const (
		fieldID  = pkg + ":id/dropped_data_view"
		expected = `ClipData { text/plain "" {T:Data text} }`
	)

	if err := d.Object(ui.ID(fieldID), ui.Text(expected)).WaitForExists(ctx, 30*time.Second); err != nil {
		actual, terr := d.Object(ui.ID(fieldID)).GetText(ctx)
		if terr != nil {
			s.Fatal("Failed to get the text: ", terr)
		}
		s.Fatalf("Unexpected drag and drop result: %v: got %q; want %q", err, actual, expected)
	}
}
