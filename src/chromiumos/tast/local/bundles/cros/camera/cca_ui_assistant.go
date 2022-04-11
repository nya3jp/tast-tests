// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type assistantOptions struct {
	Query              string
	Mode               cca.Mode
	ShouldStartCapture bool
	ExpectedFacing     cca.Facing
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIAssistant,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests opening Camera app using an Assistant query",
		Contacts: []string{
			"pihsun@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "camera_app", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Fixture:      "ccaTestBridgeReady",
	})
}

// CCAUIAssistant tests that the Camera app can be opened by the Assistant.
func CCAUIAssistant(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome
	tb := s.FixtValue().(cca.FixtureData).TestBridge()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	defer func() {
		if err := assistant.Cleanup(cleanupCtx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	for _, tc := range []struct {
		name    string
		Options assistantOptions
	}{
		{
			name: "take_photo",
			Options: assistantOptions{
				Query:              "take a photo",
				Mode:               cca.Photo,
				ShouldStartCapture: true,
			},
		},
		{
			name: "take_selfie",
			Options: assistantOptions{
				Query:              "take a selfie",
				Mode:               cca.Photo,
				ShouldStartCapture: true,
				ExpectedFacing:     cca.FacingFront,
			},
		},
		{
			name: "open_cca_photo_mode",
			Options: assistantOptions{
				Query:              "open camera",
				Mode:               cca.Photo,
				ShouldStartCapture: false,
			},
		},
		{
			name: "start_record_video",
			Options: assistantOptions{
				Query:              "record video",
				Mode:               cca.Video,
				ShouldStartCapture: true,
			},
		},
		{
			name: "open_cca_video_mode",
			Options: assistantOptions{
				Query:              "open video camera",
				Mode:               cca.Video,
				ShouldStartCapture: false,
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			if err := cca.ClearSavedDir(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			startTime := time.Now()
			options := tc.Options

			app, err := launchAssistant(ctx, cr, options, scripts, outDir, tb)
			if err != nil {
				s.Fatal("Failed to launch assistant: ", err)
			}

			defer func(ctx context.Context) {
				if err := app.Close(ctx); err != nil {
					s.Fatal("Failed to close CCA: ", err)
				}
			}(ctx)

			if err := app.CheckMode(ctx, options.Mode); err != nil {
				s.Fatal("Failed to check mode: ", err)
			}

			if options.ExpectedFacing != "" {
				if err := app.CheckCameraFacing(ctx, options.ExpectedFacing); err != nil {
					s.Fatal("Failed to check facing: ", err)
				}
			}

			if options.ShouldStartCapture {
				if options.Mode == cca.Video {
					if err := app.WaitForState(ctx, "recording", true); err != nil {
						s.Fatal("Recording is not started: ", err)
					}
					// Wait video recording for 1 second to simulate user taking a 1
					// second long video.
					if err := testing.Sleep(ctx, 1*time.Second); err != nil {
						s.Fatal("Failed to sleep for 1 second: ", err)
					}
					testing.ContextLog(ctx, "Stopping recording")
					if err := app.ClickShutter(ctx); err != nil {
						s.Fatal("Failed to click shutter button: ", err)
					}
				}
				if err := checkAssistantCaptureResult(ctx, app, options.Mode, startTime); err != nil {
					s.Fatal("Failed to check capture result: ", err)
				}
			}
		})
	}
}

// launchAssistant launches CCA intent with different options.
func launchAssistant(ctx context.Context, cr *chrome.Chrome, options assistantOptions, scripts []string, outDir string, tb *testutil.TestBridge) (*cca.App, error) {
	launchByAssistant := func(ctx context.Context, tconn *chrome.TestConn) error {
		_, err := assistant.SendTextQuery(ctx, tconn, options.Query)
		return err
	}

	return cca.Init(ctx, cr, scripts, outDir, testutil.AppLauncher{
		LaunchApp:    launchByAssistant,
		UseSWAWindow: false,
	}, tb)
}

func checkAssistantCaptureResult(ctx context.Context, app *cca.App, mode cca.Mode, startTime time.Time) error {
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get CCA default saved path")
	}
	testing.ContextLog(ctx, "Waiting for capture result")
	var filePattern *regexp.Regexp
	if mode == cca.Video {
		filePattern = cca.VideoPattern
	} else {
		filePattern = cca.PhotoPattern
	}
	_, err = app.WaitForFileSaved(ctx, dir, filePattern, startTime)
	if err != nil {
		return err
	}
	return nil
}
