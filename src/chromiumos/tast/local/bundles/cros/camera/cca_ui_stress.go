// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
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
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Vars: []string{
			// Number of iterations to test.
			"iterations",
			// Skip first skip_iterations iterations for reproducing failures faster.
			"skip_iterations",
			// The seed for deterministically generating the random action sequence.
			"seed",
		},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			ExtraAttr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
			Pre:               chrome.LoggedIn(),
			Timeout:           5 * time.Minute,
			Val:               false, // useFakeCamera
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			ExtraAttr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
			Pre:               chrome.LoggedIn(),
			Timeout:           5 * time.Minute,
			Val:               false, // useFakeCamera
		}, {
			Name:      "fake",
			ExtraAttr: []string{"group:mainline", "informational", "group:camera-libcamera"},
			Pre:       testutil.ChromeWithFakeCamera(),
			Timeout:   5 * time.Minute,
			Val:       true, // useFakeCamera
		}, {
			// For stress testing manually with real camera and longer timeout.
			Name:              "manual",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               chrome.LoggedIn(),
			Timeout:           30 * 24 * time.Hour,
			Val:               false, // useFakeCamera
		}},
	})
}

type stressAction struct {
	name    string
	perform func(context.Context) error
}

func getIntVar(s *testing.State, name string, defaultValue int) int {
	str, ok := s.Var(name)
	if !ok {
		return defaultValue
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		s.Fatalf("Failed to parse integer variable %v: %v", name, err)
	}

	return val
}

func CCAUIStress(ctx context.Context, s *testing.State) {
	const defaultIterations = 20
	const defaultSkipIterations = 0
	const defaultSeed = 1
	const perIterationTimeout = 10 * time.Second
	const cleanupTimeout = 20 * time.Second

	iterations := getIntVar(s, "iterations", defaultIterations)
	skipIterations := getIntVar(s, "skip_iterations", defaultSkipIterations)

	seed := getIntVar(s, "seed", defaultSeed)
	rand.Seed(int64(seed))

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTimeout)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, s.Param().(bool))
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(cleanupCtx)

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
	}(cleanupCtx)

	// TODO(b/182248415): Add variables to control per action parameters, like
	// how many photo should be taken consecutively or how long the video
	// recording should be.

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
		s.Fatal("Failed to get number of cameras: ", err)
	}
	if numCameras > 1 {
		actions = append(actions, stressAction{
			name: "switch-camera",
			perform: func(ctx context.Context) error {
				return app.SwitchCamera(ctx)
			},
		})
	}

	s.Logf("Start stressing for %v iterations with seed = %v, skipIterations = %v", iterations, seed, skipIterations)

	// TODO(b/182248415): Clear camera/ folder periodically, otherwise the disk
	// might be full after running many iterations.
	for i := 1; i <= iterations; i++ {
		action := actions[rand.Intn(len(actions))]
		if i <= skipIterations {
			// We still need to call rand.Intn() to advance the internal state of PRNG.
			continue
		}
		s.Logf("Iteration %d/%d: Performing action %s", i, iterations, action.name)
		actionCtx, actionCancel := context.WithTimeout(ctx, perIterationTimeout)
		if err := action.perform(actionCtx); err != nil {
			s.Fatalf("Failed to perform action %v: %v", action.name, err)
		}
		actionCancel()
	}
}
