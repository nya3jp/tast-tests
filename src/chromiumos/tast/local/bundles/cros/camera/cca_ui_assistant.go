// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type assistantBehavior struct {
	ShouldStartCapture bool
}

type assistantOptions struct {
	Query        string
	Mode         cca.Mode
	TestBehavior assistantBehavior
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CCAUIAssistant,
		Desc: "Tests opening Camera app using an Assistant query",
		Contacts: []string{
			"pihsun@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "camera_app"},
		Data:         []string{"cca_ui.js"},
		Fixture:      "ccaTestBridgeReady",
	})
}

// CCAUIAssistant tests that the Camera app can be opened by the Assistant
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
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	subTestTimeout := 20 * time.Second
	for _, tc := range []struct {
		Name    string
		Options assistantOptions
	}{
		{
			Name: "take photo",
			Options: assistantOptions{
				Query: "take a photo",
				Mode:  cca.Photo,
				TestBehavior: assistantBehavior{
					ShouldStartCapture: true,
				},
			},
		},
		{
			Name: "take photo in square mode",
			Options: assistantOptions{
				Query: "take a square photo",
				Mode:  cca.Square,
				TestBehavior: assistantBehavior{
					ShouldStartCapture: true,
				},
			},
		},
		{
			Name: "open CCA in photo mode",
			Options: assistantOptions{
				Query: "open camera",
				Mode:  cca.Photo,
				TestBehavior: assistantBehavior{
					ShouldStartCapture: false,
				},
			},
		},
		{
			Name: "start record video",
			Options: assistantOptions{
				Query: "record video",
				Mode:  cca.Video,
				TestBehavior: assistantBehavior{
					ShouldStartCapture: true,
				},
			},
		},
		{
			Name: "open CCA in video mode",
			Options: assistantOptions{
				Query: "open video camera",
				Mode:  cca.Video,
				TestBehavior: assistantBehavior{
					ShouldStartCapture: false,
				},
			},
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.Name, func(ctx context.Context, s *testing.State) {
			if err := cca.ClearSavedDir(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := checkAssistantBehavior(ctx, cr, tc.Options, scripts, outDir, tb); err != nil {
				s.Error("Failed when checking assistant behavior: ", err)
			}
		})
		cancel()
	}
}

// launchAssistant launches CCA intent with different options.
func launchAssistant(ctx context.Context, cr *chrome.Chrome, options assistantOptions, scripts []string, outDir string, tb *testutil.TestBridge) (*cca.App, error) {
	launchByAssistant := func(ctx context.Context, tconn *chrome.TestConn) error {
		if _, err := assistant.SendTextQuery(ctx, tconn, options.Query); err != nil {
			return err
		}
		return nil
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
	testing.ContextLog(ctx, "Checking capture result")
	var filePattern *regexp.Regexp
	if mode == cca.Video {
		filePattern = cca.VideoPattern
	} else {
		filePattern = cca.PhotoPattern
	}
	fileInfo, err := app.WaitForFileSaved(ctx, dir, filePattern, startTime)
	if err != nil {
		return err
	} else if fileInfo.Size() == 0 {
		return errors.New("capture result is empty")
	}
	return nil
}

// checkAssistantBehavior checks basic control flow for launching with assistant with different queries.
func checkAssistantBehavior(ctx context.Context, cr *chrome.Chrome, options assistantOptions, scripts []string, outDir string, tb *testutil.TestBridge) (retErr error) {
	startTime := time.Now()

	app, err := launchAssistant(ctx, cr, options, scripts, outDir, tb)
	if err != nil {
		return err
	}

	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			} else {
				retErr = err
			}
		}
	}(ctx)

	if err := app.CheckLandingMode(ctx, options.Mode); err != nil {
		return err
	}

	if options.TestBehavior.ShouldStartCapture {
		if options.Mode == cca.Video {
			if err := app.WaitForState(ctx, "recording", true); err != nil {
				return errors.Wrap(err, "recording is not started")
			}
			if err := testing.Sleep(ctx, 3*time.Second); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Stopping recording")
			if err := app.ClickShutter(ctx); err != nil {
				return errors.Wrap(err, "failed to click shutter button")
			}
		}
		if err := checkAssistantCaptureResult(ctx, app, options.Mode, startTime); err != nil {
			return err
		}
	}

	return nil
}
