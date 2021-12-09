// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Data:         []string{"face_ae_60_90_20211209.jpg"},
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

// calibrateDisplayLevel finds the chart display level which makes luma value of face area falls in [targetLumaMin, targetLumaMax].
// The luma value is measured without faceAE enabled.
func calibrateDisplayLevel(ctx context.Context, ch *chart.Helper, d *dut.DUT, facing pb.Facing, outDir string, targetLumaMin, targetLumaMax int64) (float32, error) {
	displayLevel := float32(50.0)
	adjustStep := float32(10.0)
	prevDisplayLevel := displayLevel
	for {
		err := ch.SetDisplayLevel(displayLevel)
		if err != nil {
			return 0.0, err
		}
		luma, err := face.GetFaceLumaValue(ctx, d, facing, false, outDir, fmt.Sprintf("displayLevel_%f", displayLevel))
		if err != nil {
			return 0.0, err
		}
		testing.ContextLogf(ctx, "display level %f, luma %d", displayLevel, luma)
		if luma >= targetLumaMin && luma <= targetLumaMax {
			return displayLevel, nil
		}
		if luma < targetLumaMin {
			if displayLevel+adjustStep == prevDisplayLevel {
				return 0.0, errors.New("display level tested twice")
			}
			prevDisplayLevel = displayLevel
			displayLevel += adjustStep
		}
		if luma > targetLumaMax {
			if displayLevel-adjustStep == prevDisplayLevel {
				return 0.0, errors.New("display level tested twice")
			}
			prevDisplayLevel = displayLevel
			if displayLevel <= 10 {
				displayLevel -= 2
			} else {
				displayLevel -= adjustStep
			}
		}
		// displayLevel 0 means whole black
		if displayLevel == 0 || displayLevel > 100 {
			return 0.0, errors.New("display level out of range")
		}
	}
	return 0.0, errors.New("can't find display level in calibration")
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

	chartPath := s.DataPath("face_ae_60_90_20211209.jpg")
	outDir := s.OutDir()
	// log the test scene by default display level.
	ch, err := chart.NewHelper(ctx, d, altAddr, chartPath, outDir, chart.DisplayDefaultLevel)
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer ch.Close()
	if err := camerabox.LogTestScene(ctx, d, facing, outDir); err != nil {
		s.Error("Failed to take a photo of test scene: ", err)
	}

	displayLevel, err := calibrateDisplayLevel(ctx, ch, d, facing, outDir, 60, 70)
	if err != nil {
		s.Fatal("Failed to calibrate display level: ", err)
	}
	s.Logf("Use display level %f after calibration", displayLevel)

	// Check face auto exposure luma value.
	err = ch.SetDisplayLevel(displayLevel)
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	luma, err := face.GetFaceLumaValue(ctx, d, facing, false, outDir, fmt.Sprintf("displayLevel_%f", displayLevel))
	if err != nil {
		s.Fatal("Failed to get face luma value: ", err)
	}
	s.Logf("Face auto exposure luma %d", luma)
	if !(luma >= 90 && luma <= 120) {
		s.Errorf("Luma value %d of face auto exposure is not in range [90, 120]", luma)
	}
}
