// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImageDrop,
		Desc:         "Checks image drag drop app compat from browser",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"image_paste_manifest.json", "image_paste_background.js", "image_paste_foreground.html", "image_paste_sample.png"},
		Timeout:      4 * time.Minute,
	})
}

func ImageDrop(ctx context.Context, s *testing.State) {
	s.Log("Copying extension to temp directory")
	extDir, err := ioutil.TempDir("", "tast.arc.ImagePasteExtension")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)
	for _, name := range []string{"manifest.json", "background.js", "foreground.html", "sample.png"} {
		if err := fsutil.CopyFile(s.DataPath("image_paste_"+name), filepath.Join(extDir, name)); err != nil {
			s.Fatalf("Failed to copy extension %s: %v", name, err)
		}
	}

	// TODO(tetsui): Remove the flag once it's enabled by default.
	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir), chrome.ARCEnabled(), chrome.EnableFeatures("ArcImageCopyPasteCompat"), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	const (
		apk          = "ArcImagePasteTest.apk"
		pkg          = "org.chromium.arc.testapp.imagepaste"
		activityName = ".MainActivity"
		fieldID      = pkg + ":id/input_field"
		counterID    = pkg + ":id/counter"
	)

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}
	fgURL := "chrome-extension://" + extID + "/foreground.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fgURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", fgURL, err)
	}
	defer conn.Close()

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}
	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	// Focus the input field and drop the image.
	if err := d.Object(ui.ID(fieldID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the input field: ", err)
	}

	rect, err := d.Object(ui.ID(fieldID)).GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the input field bounds: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	dispMode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode: ", err)
	}

	rect = coords.ConvertBoundsFromPXToDP(rect, dispMode.DeviceScaleFactor)


	if err := mouse.Drag(ctx, tconn, coords.Point{50, 50}, rect.CenterPoint(), time.Second); err != nil {
		s.Fatal("Failed to send drag events: ", err)
	}

	if err := act.Focus(ctx, tconn); err != nil {
		s.Fatal("Failed to focus on the activity: ", err)
	}

	// Verify the image is pasted successfully by checking the counter.
	if err := d.Object(ui.ID(counterID)).WaitForText(ctx, "1", 30*time.Second); err != nil {
		s.Fatal("Failed to paste the image: ", err)
	}
}
