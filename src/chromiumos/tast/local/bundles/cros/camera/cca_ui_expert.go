// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"sort"
	"strings"

	"chromiumos/tast/errors"
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
	defer app.RemoveCacheData(ctx,
		[]string{"expert", "showMetadata", "saveMetadata"})

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")

	for i, action := range []struct {
		Name    string
		Func    func(context.Context, *cca.App) error
		Enabled bool
	}{
		// Expert mode is not reset after each test for persistency
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertModeOptions", toggleExpertModeOptions, true},
		{"switchSquareMode", switchSquareMode, true},
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertMode", toggleExpertMode, true},
		{"toggleExpertModeOptions", toggleExpertModeOptions, false},
	} {
		if err := action.Func(ctx, app); err != nil {
			s.Fatalf("Failed to perform action %v of test %v: %v", action.Name, i, err)
		}
		if err := verifyExpertMode(ctx, app, action.Enabled); err != nil {
			s.Errorf("Failed in test %v %v(): %v", i, action.Name, err)
		}
	}
}

func verifyExpertMode(ctx context.Context, app *cca.App, enabled bool) error {
	verifyShowMetadata := func(ctx context.Context, enabled bool) error {
		return app.CheckMetadataVisibility(ctx, enabled)
	}

	verifySaveMetadata := func(ctx context.Context, enabled bool) error {
		if fileInfos, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			return err
		} else if !checkSavedMetadata(fileInfos, enabled) {
			return errors.New("failed to save metadata correctly")
		}
		return nil
	}

	if err := verifyShowMetadata(ctx, enabled); err != nil {
		return err
	}
	if err := verifySaveMetadata(ctx, enabled); err != nil {
		return err
	}
	return nil
}

func checkSavedMetadata(fileInfos []os.FileInfo, enabled bool) bool {
	numFiles := len(fileInfos)
	if numFiles%2 != 0 {
		return !enabled
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Name() < fileInfos[j].Name()
	})

	for i := 0; i < numFiles; i += 2 {
		file1 := strings.Split(fileInfos[i].Name(), ".")
		file2 := strings.Split(fileInfos[i+1].Name(), ".")
		if file1[0] != file2[0] || file1[1] != "jpg" || file2[1] != "json" {
			return !enabled
		}
	}

	return enabled
}

func toggleExpertMode(ctx context.Context, app *cca.App) error {
	_, err := app.ToggleExpertMode(ctx)
	return err
}

func toggleExpertModeOptions(ctx context.Context, app *cca.App) error {
	if _, err := app.ToggleShowMetadata(ctx); err != nil {
		return err
	}
	if _, err := app.ToggleSaveMetadata(ctx); err != nil {
		return err
	}
	return nil
}

func switchSquareMode(ctx context.Context, app *cca.App) error {
	return app.SwitchMode(ctx, cca.Square)
}

func restart(ctx context.Context, app *cca.App) error {
	if err := app.Restart(ctx); err != nil {
		return err
	}
	return app.WaitForVideoActive(ctx)
}
