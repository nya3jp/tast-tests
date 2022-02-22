// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/camera/camerabox"
	"chromiumos/tast/remote/bundles/cros/camera/face"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FaceAutoExposure,
		Desc:         "Verifies face auto exposure",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinUSBCamera},
		Data:         []string{"te273_mia_20211228.jpg"},
		Vars:         []string{"chart"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       pb.Facing_FACING_BACK,
			},
			{
				Name:      "front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       pb.Facing_FACING_FRONT,
			},
		},
	})
}

func getFaceLumaValue(ctx context.Context, d *dut.DUT, facing pb.Facing, enableFaceAe bool, outDir string, displayLevel float32) (int64, error) {
	return face.GetFaceLumaValue(ctx, d, facing, enableFaceAe, outDir, fmt.Sprintf("displayLevel_%.0f", displayLevel))
}

// calibrateDisplayLevel finds the chart display level which can let face to be detected.
// Returns (display level, luma value of face image, error)
func calibrateDisplayLevel(ctx context.Context, ch *chart.Chart, d *dut.DUT, facing pb.Facing, outDir string) (float32, int64, error) {
	adjustStep := float32(10.0)
	for displayLevel := float32(20.0); displayLevel <= 100.0; displayLevel += adjustStep {
		if ctx.Err() != nil {
			return 0.0, 0, errors.Errorf("Context already expired: %s", ctx.Err())
		}
		testing.ContextLogf(ctx, "Try Display level %f", displayLevel)
		if err := ch.SetDisplayLevel(ctx, displayLevel); err != nil {
			return 0.0, 0, err
		}
		// It may not find face when the display level is not good enough and it returns error.
		// We should try next display level when there is a error.
		luma, err := getFaceLumaValue(ctx, d, facing, false, outDir, displayLevel)
		if err == nil && luma > 0 {
			testing.ContextLogf(ctx, "Find Display level %f, luma %d", displayLevel, luma)
			return displayLevel, luma, nil
		}
	}
	return 0.0, 0, errors.New("can't find display level in calibration")
}

func FaceAutoExposure(ctx context.Context, s *testing.State) {
	d := s.DUT()
	facing := s.Param().(pb.Facing)

	roiSupport, err := face.CheckRoiSupport(ctx, d, facing)
	if err != nil {
		s.Fatal("Failed to check ROI support: ", err)
	}

	if !roiSupport {
		s.Log("Skip this DUT, because it doesn't support roi")
		return
	}

	var altAddr string
	if chartAddr, ok := s.Var("chart"); ok {
		altAddr = chartAddr
	}

	outDir := s.OutDir()
	// Log the test scene by default display level.
	ch, namePaths, err := chart.New(ctx, d, altAddr, outDir, []string{s.DataPath("te273_mia_20211228.jpg")})
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer ch.Close(ctx, outDir)
	if err := ch.Display(ctx, namePaths[0]); err != nil {
		s.Fatal("Failed to display chart on chart tablet: ", err)
	}
	if err := camerabox.LogTestScene(ctx, d, facing, outDir); err != nil {
		s.Error("Failed to take a photo of test scene: ", err)
	}

	displayLevel, luma, err := calibrateDisplayLevel(ctx, ch, d, facing, outDir)
	if err != nil {
		s.Fatal("Failed to calibrate display level: ", err)
	}
	s.Logf("Use display level %f after calibration", displayLevel)

	lumaFD, err := getFaceLumaValue(ctx, d, facing, true, outDir, displayLevel)
	if err != nil {
		s.Fatal("Failed to get face luma value: ", err)
	}

	// We verify the function by checking the luma value of face detection enable/disable.
	// The luma value of face detectoin enabled should be more than 10% of the disable one.
	targetMinLuma := int64(float64(luma) * 1.1)
	s.Logf("Face auto exposure luma %d, targetMinLuma %d", lumaFD, targetMinLuma)
	if lumaFD < targetMinLuma {
		s.Errorf("Luma value %d of face auto exposure is not bigger than %d", lumaFD, targetMinLuma)
	}
}
