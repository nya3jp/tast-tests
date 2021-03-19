// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type qrcodeTestParams struct {
	format     string
	expected   string
	chip       cca.UIComponent
	copyButton cca.UIComponent
	canOpen    bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIQRCode,
		Desc:         "Checks QR code detection in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "chrome_internal"},
		Params: []testing.Param{{
			Name:    "url",
			Fixture: "ccaLaunchedWithQRCodeUrlScene",
			Val: qrcodeTestParams{
				format:     "url",
				expected:   "https://www.google.com/chromebook/chrome-os/",
				chip:       cca.BarcodeChipURL,
				copyButton: cca.BarcodeCopyURLButton,
				canOpen:    true,
			},
		}, {
			Name:    "text",
			Fixture: "ccaLaunchedWithQRCodeTextScene",
			Val: qrcodeTestParams{
				format:     "text",
				expected:   "Chrome OS is the speedy, simple and secure operating system that powers every Chromebook.",
				chip:       cca.BarcodeChipText,
				copyButton: cca.BarcodeCopyTextButton,
				canOpen:    false,
			},
		}},
	})
}

func CCAUIQRCode(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	cr := s.FixtValue().(cca.FixtureData).Chrome
	testParams := s.Param().(qrcodeTestParams)

	enabled, err := app.ToggleQRCodeOption(ctx)
	if err != nil {
		s.Fatal("Failed to enable QR code detection: ", err)
	}
	if !enabled {
		s.Fatal("QR code detection is not enabled after toggling")
	}
	s.Log("Start scanning QR Code")

	if err := app.WaitForVisibleState(ctx, testParams.chip, true); err != nil {
		s.Fatalf("Failed to detect %v from barcode: %v", testParams.format, err)
	}
	s.Logf("%v detected", testParams.format)

	// Copy the content.
	if err := app.Click(ctx, testParams.copyButton); err != nil {
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
		if clipData != testParams.expected {
			return errors.Errorf("unexpected clipboard data: got %q, want %q", clipData, testParams.expected)
		}
		s.Logf("%v copied successfully", testParams.format)
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to get expected clipboard data: ", err)
	}

	if testParams.canOpen {
		if err := app.Click(ctx, testParams.chip); err != nil {
			s.Fatal("Failed to click chip: ", err)
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ok, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(testParams.expected))
			if err != nil {
				return testing.PollBreak(err)
			}
			if !ok {
				return errors.Errorf("no match target for %v", testParams.expected)
			}
			s.Logf("%v opened successfully", testParams.format)
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Failed to open: ", err)
		}
	}
	// TODO(b/172879638): Test invalid binary content.
}
