// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImageDropFromDownloads,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks image drag drop app compat from Files App",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBootedInClamshellMode",
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"capybara.jpg"},
		Timeout:      4 * time.Minute,
	})
}

func ImageDropFromDownloads(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	const filename = "capybara.jpg"

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.NormalizedUser(), err)
	}
	crosPath := filepath.Join(cryptohomeUserPath, "MyFiles", "Downloads", filename)

	expected, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatal("Could not read the test file: ", err)
	}

	if err = ioutil.WriteFile(crosPath, expected, 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", crosPath, err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard")
	}
	defer keyboard.Close()

	const (
		apk          = "ArcImagePasteTest.apk"
		pkg          = "org.chromium.arc.testapp.imagepaste"
		activityName = ".MainActivity"
		fieldID      = pkg + ":id/input_field"
		counterID    = pkg + ":id/counter"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}
	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	if _, err := ash.SetARCAppWindowStateAndWait(ctx, tconn, pkg, ash.WindowStateRightSnapped); err != nil {
		s.Fatal("Failed to snap the activity: ", err)
	}

	// Focus the input field and drag the image.
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

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed launching the Files App: ", err)
	}
	if err := uiauto.Combine("drag and drop capybara.jpg from Downloads",
		keyboard.AccelAction("Alt+["),
		files.OpenDownloads(),
		files.DragAndDropFile(filename, rect.CenterPoint(), keyboard),
	)(ctx); err != nil {
		s.Fatal("Failed to open the Downloads directory: ", err)
	}

	if err := act.Focus(ctx, tconn); err != nil {
		s.Fatal("Failed to focus on the activity: ", err)
	}

	// Verify the image is dropped successfully by checking the counter.
	if err := d.Object(ui.ID(counterID)).WaitForText(ctx, "1", 30*time.Second); err != nil {
		s.Fatal("Failed to paste the image: ", err)
	}
}
