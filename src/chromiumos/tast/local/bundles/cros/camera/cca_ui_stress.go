// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIStress,
		Desc:         "Opens CCA and stress testing common functions randomly",
		Contacts:     []string{"shik@chromium.org", "inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"iterations"},
		// The timeout would be shorten in the test according to the number of iterations
		Timeout: 30 * 24 * time.Hour,
	})
}

type stressAction struct {
	name    string
	perform func(context.Context) error
}

func getIteration(s *testing.State) int {
	const defaultIterations = 20

	iterationsVar, ok := s.Var("iterations")
	if !ok {
		return defaultIterations
	}

	iterations, err := strconv.Atoi(iterationsVar)
	if err != nil {
		s.Fatal("Failed to parse iterations variable: ", err)
	}
	return iterations
}

func CCAUIStress(ctx context.Context, s *testing.State) {
	const perIterationTimeout = 10 * time.Second

	iterations := getIteration(s)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(iterations)*perIterationTimeout)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)
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

	actions := []stressAction{
		{
			name: "restart-app",
			perform: func(ctx context.Context) error {
				return app.Restart(ctx, tb)
			},
		},
		{
			name: "take-photo",
			perform: func(ctx context.Context) error {
				if err := app.SwitchMode(ctx, cca.Photo); err != nil {
					return err
				}
				_, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
				return err
			},
		},
		{
			name: "record-video",
			perform: func(ctx context.Context) error {
				if err := app.SwitchMode(ctx, cca.Video); err != nil {
					return err
				}
				_, err := app.RecordVideo(ctx, cca.TimerOff, 3*time.Second)
				return err
			},
		},
	}

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}
	if numCameras > 1 {
		actions = append(actions, stressAction{
			name: "switch-camera",
			perform: func(ctx context.Context) error {
				return app.SwitchCamera(ctx)
			},
		})
	}

	for i := 1; i <= iterations; i++ {
		action := actions[rand.Intn(len(actions))]
		s.Logf("Iteration %d/%d: Performing action %s", i, iterations, action.name)
		actionCtx, cancel := context.WithTimeout(ctx, perIterationTimeout)
		if err := action.perform(actionCtx); err != nil {
			s.Fatalf("Failed to perfoce action %v: %v", action.name, err)
		}
		cancel()
	}
}
