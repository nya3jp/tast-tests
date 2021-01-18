// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIQRCode,
		Desc:         "Checks QR code detection in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"cca_ui.js", "qrcode_1280x960.y4m"},
	})
}

func CCAUIQRCode(ctx context.Context, s *testing.State) {
	const expectedURL = "https://www.google.com/chromebook/chrome-os/"
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs(
			"--use-fake-device-for-media-stream",
			"--use-file-for-fake-video-capture="+s.DataPath("qrcode_1280x960.y4m")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test connection: ", err)
	}

	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	enabled, err := app.ToggleQRCodeOption(ctx)
	if err != nil {
		s.Fatal("Failed to enable QR code detection: ", err)
	}
	if !enabled {
		s.Fatal("QR code detection is not enabled after toggling")
	}
	s.Log("Start scanning QR Code")

	if err := app.WaitForVisibleState(ctx, cca.BarcodeChipURL, true); err != nil {
		s.Fatal("Failed to detect url from barcode: ", err)
	}
	s.Log("URL detected")

	// Copy the url.
	if err := app.Click(ctx, cca.BarcodeCopyURLButton); err != nil {
		s.Fatal("Failed to click copy button: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var clipData string
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return testing.PollBreak(err)
		}
		if clipData != expectedURL {
			return errors.Errorf("unexpected clipboard data: got %q, want %q", clipData, expectedURL)
		}
		s.Log("URL copied successfully")
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to get expected clipboard data: ", err)
	}

	// Open the url.
	if err := app.Click(ctx, cca.BarcodeChipURL); err != nil {
		s.Fatal("Failed to click url: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ok, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(expectedURL))
		if err != nil {
			return testing.PollBreak(err)
		}
		if !ok {
			return errors.Errorf("no match target for %v", expectedURL)
		}
		s.Log("URL opened successfully")
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to open url: ", err)
	}

	// TODO(b/172879638): Test text chip.
	// TODO(b/172879638): Test invalid binary content.
}
