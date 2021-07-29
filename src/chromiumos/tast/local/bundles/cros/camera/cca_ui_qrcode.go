// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIQRCode,
		Desc:         "Checks QR code detection in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "chrome_internal"},
		Data:         []string{"cca_ui.js", "qrcode_1280x960.y4m", "qrcode_text_1280x960.y4m"},
	})
}

func CCAUIQRCode(ctx context.Context, s *testing.State) {
	subTestTimeout := 60 * time.Second
	for _, tc := range []struct {
		format     string
		video      string
		expected   string
		chip       cca.UIComponent
		copyButton cca.UIComponent
		canOpen    bool
	}{
		{
			format:     "url",
			video:      "qrcode_1280x960.y4m",
			expected:   "https://www.google.com/chromebook/chrome-os/",
			chip:       cca.BarcodeChipURL,
			copyButton: cca.BarcodeCopyURLButton,
			canOpen:    true,
		},
		{
			format:     "text",
			video:      "qrcode_text_1280x960.y4m",
			expected:   "Chrome OS is the speedy, simple and secure operating system that powers every Chromebook.",
			chip:       cca.BarcodeChipText,
			copyButton: cca.BarcodeCopyTextButton,
			canOpen:    false,
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.format, func(ctx context.Context, s *testing.State) {
			cr, err := chrome.New(ctx,
				chrome.ExtraArgs(
					"--use-fake-device-for-media-stream",
					"--use-file-for-fake-video-capture="+s.DataPath(tc.video)))
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}

			tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseFakeCamera)
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

			if err := app.WaitForVisibleState(ctx, tc.chip, true); err != nil {
				s.Fatalf("Failed to detect %v from barcode: %v", tc.format, err)
			}
			s.Logf("%v detected", tc.format)

			// Copy the content.
			if err := app.Click(ctx, tc.copyButton); err != nil {
				s.Fatal("Failed to click copy button: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get test connection: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var clipData string
				if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
					return testing.PollBreak(err)
				}
				if clipData != tc.expected {
					return errors.Errorf("unexpected clipboard data: got %q, want %q", clipData, tc.expected)
				}
				s.Logf("%v copied successfully", tc.format)
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to get expected clipboard data: ", err)
			}

			if tc.canOpen {
				if err := app.Click(ctx, tc.chip); err != nil {
					s.Fatal("Failed to click chip: ", err)
				}

				if err := testing.Poll(ctx, func(ctx context.Context) error {
					ok, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(tc.expected))
					if err != nil {
						return testing.PollBreak(err)
					}
					if !ok {
						return errors.Errorf("no match target for %v", tc.expected)
					}
					s.Logf("%v opened successfully", tc.format)
					return nil
				}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
					s.Fatal("Failed to open: ", err)
				}
			}
		})
		cancel()
	}
	// TODO(b/172879638): Test invalid binary content.
}
