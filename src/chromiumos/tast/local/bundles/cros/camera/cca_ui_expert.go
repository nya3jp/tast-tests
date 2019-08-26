// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIExpert,
		Desc:         "Opens CCA and verifies the expert options",
		Contacts:     []string{"kaihsien@google.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "arc_camera3", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIExpert(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")

	restartApp := func() {
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			s.Fatal("Preview is inactive after restart App: ", err)
		}
	}

	verifyExpertMode := func(testName string, enabled bool) {
		if visible, err := app.MetadataVisible(ctx); err != nil {
			s.Errorf("%v: Failed to check show metadata status: %v", testName, err)
		} else if visible != enabled {
			s.Errorf("%v: Metadata is not showing correctly", testName)
		}

		if fileInfos, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Errorf("%v: Failed when taking photo: %v", testName, err)
		} else if containsMetadata(fileInfos) != enabled {
			s.Errorf("%v: Metadata is not saved", testName)
		}
	}

	for i, action := range []struct {
		Name   string
		Func   func(context.Context, *cca.App) error
		Result bool
	}{
		// Expert mode is not reset after each test for persistency
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertModeOptions", toggleExpertModeOptions, true},
		{"switchPortraitMode", switchPortraitMode, true},
		{"switchSquareMode", switchSquareMode, true},
		{"switchCamera", switchCamera, true},
		{"restart", restart, true},
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertMode", toggleExpertMode, true},
		{"toggleExpertModeOptions", toggleExpertModeOptions, false},
		{"restart", restart, false},
	} {
		if err := action.Func(ctx, app); err != nil {
			s.Errorf("Failed in test %v %v(): %v", i, action.Name, err)
			restartApp()
		} else {
			verifyExpertMode(action.Name, action.Result)
		}
	}
}

func containsMetadata(fileInfos []os.FileInfo) bool {
	numFiles := len(fileInfos)
	if numFiles%2 != 0 {
		return false
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Name() < fileInfos[j].Name()
	})

	for i := 0; i < numFiles; i += 2 {
		file1 := strings.Split(fileInfos[i].Name(), ".")
		file2 := strings.Split(fileInfos[i+1].Name(), ".")
		if file1[0] != file2[0] || file1[1] != "jpg" || file2[1] != "json" {
			return false
		}
	}

	return true
}

func toggleExpertMode(ctx context.Context, app *cca.App) error {
	_, err := app.ToggleExpertMode(ctx)
	// Wait for all events in CCA to finish dispatching
	testing.Sleep(ctx, time.Second)
	return err
}

func toggleExpertModeOptions(ctx context.Context, app *cca.App) error {
	if err := app.ClickWithSelector(ctx, "#expert-show-metadata"); err != nil {
		return err
	}
	if err := app.ClickWithSelector(ctx, "#expert-save-metadata"); err != nil {
		return err
	}
	// Wait for all events in CCA to finish dispatching
	testing.Sleep(ctx, time.Second)
	return nil
}

func switchSquareMode(ctx context.Context, app *cca.App) error {
	return app.SwitchMode(ctx, cca.Square)
}

func switchPortraitMode(ctx context.Context, app *cca.App) error {
	return app.SwitchMode(ctx, cca.Portrait)
}

func switchCamera(ctx context.Context, app *cca.App) error {
	err := app.SwitchCamera(ctx)
	// Wait for all events in CCA to finish dispatching
	testing.Sleep(ctx, time.Second)
	return err
}

func restart(ctx context.Context, app *cca.App) error {
	if err := app.Restart(ctx); err != nil {
		return err
	}
	return app.WaitForVideoActive(ctx)
}
