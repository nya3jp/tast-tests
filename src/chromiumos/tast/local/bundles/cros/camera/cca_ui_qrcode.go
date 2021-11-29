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
	scene      string
	chip       cca.UIComponent
	copyButton cca.UIComponent
	canOpen    bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIQRCode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks QR code detection in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "chrome_internal"},
		Data:         []string{"qrcode_1280x960.mjpeg", "qrcode_text_1280x960.mjpeg"},
		Fixture:      "ccaTestBridgeReadyWithFakeCamera",
	})
}

func CCAUIQRCode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp
	switchScene := s.FixtValue().(cca.FixtureData).SwitchScene

	subTestTimeout := 30 * time.Second
	for _, tst := range []struct {
		name       string
		scene      string
		testParams qrcodeTestParams
	}{{
		"url",
		"qrcode_1280x960.mjpeg",
		qrcodeTestParams{
			format:     "url",
			expected:   "https://www.google.com/chromebook/chrome-os/",
			chip:       cca.BarcodeChipURL,
			copyButton: cca.BarcodeCopyURLButton,
			canOpen:    true,
		},
	}, {
		"text",
		"qrcode_text_1280x960.mjpeg",
		qrcodeTestParams{
			format:     "text",
			expected:   "Chrome OS is the speedy, simple and secure operating system that powers every Chromebook.",
			chip:       cca.BarcodeChipText,
			copyButton: cca.BarcodeCopyTextButton,
			canOpen:    false,
		},
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := switchScene(s.DataPath(tst.scene)); err != nil {
				s.Fatal("Failed to setup QRCode scene: ", err)
			}
			if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
				return runQRCodeTest(ctx, cr, app, tst.testParams)
			}, cca.TestWithAppParams{}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}
}

func runQRCodeTest(ctx context.Context, cr *chrome.Chrome, app *cca.App, testParams qrcodeTestParams) error {
	if err := app.EnableQRCodeDetection(ctx); err != nil {
		return errors.Wrap(err, "failed to enable QR code detection")
	}
	testing.ContextLog(ctx, "Start scanning QR Code")

	if err := app.WaitForVisibleState(ctx, testParams.chip, true); err != nil {
		return errors.Wrapf(err, "failed to detect %v from barcode", testParams.format)
	}
	testing.ContextLogf(ctx, "%v detected", testParams.format)

	// Copy the content.
	if err := app.Click(ctx, testParams.copyButton); err != nil {
		return errors.Wrap(err, "failed to click copy button")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test connection")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var clipData string
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return testing.PollBreak(err)
		}
		if clipData != testParams.expected {
			return errors.Errorf("unexpected clipboard data: got %q, want %q", clipData, testParams.expected)
		}
		testing.ContextLogf(ctx, "%v copied successfully", testParams.format)
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to get expected clipboard data")
	}

	if testParams.canOpen {
		if err := app.Click(ctx, testParams.chip); err != nil {
			return errors.Wrap(err, "failed to click chip")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ok, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(testParams.expected))
			if err != nil {
				return testing.PollBreak(err)
			}
			if !ok {
				return errors.Errorf("no match target for %v", testParams.expected)
			}
			testing.ContextLogf(ctx, "%v opened successfully", testParams.format)
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to open")
		}
	}
	// TODO(b/172879638): Test invalid binary content.
	return nil
}
