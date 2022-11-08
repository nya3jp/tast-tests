// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIA11y,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Chromevox reads Chrome Camera App elements as expected",
		Contacts:     []string{"dorahkim@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIA11y(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()

	// Shorten deadline to leave time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create EventWriter from keyboard")
	}
	defer ew.Close()

	visited := make(map[string]bool)
	tab := "Tab"

	for true {
		arialabel, err := app.ReturnFocusedElementAriaLabel(ctx)
		if err != nil {
			s.Error("Failed to get a focused node: ", err)
		}

		if visited[arialabel] {
			break
		}

		// There is a case of speaking "+" as "plus" like below.
		// expected: Document scanning now available. Search + Left arrow to access.
		// spoken: Document scanning now available. Search plus Left arrow to access.
		arialabel = strings.Replace(arialabel, "+", "plus", -1)

		visited[arialabel] = true

		if arialabel == "Take photo" {
			if err := takePictureByKeyboard(ctx, ew, app); err != nil {
				s.Fatal("Failed to take a picture: ", err)
			}
		}

		if err = ew.Accel(ctx, tab); err != nil {
			s.Fatal("Failed to press tab key")
		}
	}
}

func takePictureByKeyboard(ctx context.Context, ew *input.KeyboardEventWriter, app *cca.App) error {
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get result saved directory")
	}

	start := time.Now()

	space := "Space"
	if err = ew.Accel(ctx, space); err != nil {
		return errors.Wrap(err, "failed to press space key")
	}

	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}

	if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find captured result file")
	}

	return nil
}
