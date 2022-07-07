// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"

	//"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"

	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImagePaste,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks image copy paste app compat CUJ",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"image_paste_manifest.json", "image_paste_background.js", "image_paste_foreground.html", "image_paste_sample.png"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func ImagePaste(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
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
	opts := []chrome.Option{chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell")}

	bt := s.Param().(browser.Type)
	switch bt {
	case browser.TypeLacros:
		opts = append(opts, chrome.LacrosUnpackedExtension(extDir))
	case browser.TypeAsh:
		opts = append(opts, chrome.UnpackedExtension(extDir))
	}

	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

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
	conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL(fgURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", fgURL, err)
	}
	defer conn.Close()

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	if ws, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get the window list: ", err)
	} else if len(ws) != 1 {
		s.Logf("Expected 1 window, got %v window(s), ws: %v", len(ws), ws)
		// Click Alt+Tab twice to bring focus to Lacros Extension Window.
		if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
			s.Error("Failed to hit alt-tab to focus back to extension: ", err)
		}
		if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
			s.Error("Failed to hit alt-tab to focus back to extension: ", err)
		}
	}

	// Paste an image from Chrome. clipboard.write() is only available for the focused window,
	// so we use a foreground page here.
	// TODO(tetsui): Rewrite this without a custom extension so that we can use a fixture.
	if err := conn.Call(ctx, nil, `async () => {
	  const response = await fetch('sample.png');
	  const blob = await response.blob();
	  await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
	}`); err != nil {
		s.Fatal("Failed to paste an image: ", err)
	}

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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Focus the input field and paste the image.
	if err := d.Object(ui.ID(fieldID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the input field: ", err)
	}
	if err := d.Object(ui.ID(fieldID)).Click(ctx); err != nil {
		s.Fatal("Failed to click the input field: ", err)
	}
	if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus on the input field: ", err)
	}
	if err := kb.Accel(ctx, "Ctrl+V"); err != nil {
		s.Fatal("Failed to press Ctrl+V: ", err)
	}

	// Verify the image is pasted successfully by checking the counter.
	if err := d.Object(ui.ID(counterID)).WaitForText(ctx, "1", 30*time.Second); err != nil {
		s.Fatal("Failed to paste the image: ", err)
	}
}
