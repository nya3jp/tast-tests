// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/media/caps"
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

func calibrateDisplayLevel(ctx context.Context, s *testing.State, altAddr string, targetLumaMin, targetLumaMax int64) float32 {
	d := s.DUT()
	facing := s.Param().(pb.Facing)
	displayLevel := float32(50.0)
	adjustStep := float32(10.0)
	for {
		c, err := chart.NewWithDisplayLevel(ctx, d, altAddr, s.DataPath("face_ae_60_90_20211209.jpg"), s.OutDir(), displayLevel)
		if err != nil {
			s.Fatal("Failed to prepare chart tablet: ", err)
		}
		defer c.Close(ctx, s.OutDir())
		luma := face.GetFaceLumaValue(ctx, s, facing, false, s.OutDir(), fmt.Sprintf("/tmp/calibrate_%f.i420", displayLevel))
		s.Logf("display level %f, luma %d", displayLevel, luma)
		if luma >= targetLumaMin && luma <= targetLumaMax {
			break
		}
		if luma < targetLumaMin {
			displayLevel += adjustStep
		}
		if luma > targetLumaMax {
			displayLevel -= adjustStep
		}
		// displayLevel 0 means whole black, do only checks the lowest value as 10
		if displayLevel <= 10 || displayLevel > 100 {
			s.Fatal("Failed to calibrate display level")
		}
	}
	return displayLevel
}

func FaceAutoExposure(ctx context.Context, s *testing.State) {
	d := s.DUT()
	facing := s.Param().(pb.Facing)

	// Check if this DUT has a fd enabled camera with the tested facing.
	if !face.CheckRoiSupport(ctx, s) {
		s.Log("Skip this DUT, because it doesn't support roi")
		return
	}

	var altAddr string
	if chartAddr, ok := s.Var("chart"); ok {
		altAddr = chartAddr
	}

	// log the test scene by default display level
	c, err := chart.New(ctx, d, altAddr, s.DataPath("face_ae_60_90_20211209.jpg"), s.OutDir())
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer c.Close(ctx, s.OutDir())
	if err := camerabox.LogTestScene(ctx, d, facing, s.OutDir()); err != nil {
		s.Error("Failed to take a photo of test scene: ", err)
	}

	displayLevel := calibrateDisplayLevel(ctx, s, altAddr, 60, 70)
	s.Logf("Use display level %f after calibration", displayLevel)

	// Check face auto exposure luma value
	c, err = chart.NewWithDisplayLevel(ctx, d, altAddr, s.DataPath("face_ae_60_90_20211209.jpg"), s.OutDir(), displayLevel)
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer c.Close(ctx, s.OutDir())
	luma := face.GetFaceLumaValue(ctx, s, facing, true, s.OutDir(), fmt.Sprintf("/tmp/calibrate_%f.i420", displayLevel))
	s.Logf("Face auto exposure luma %d", luma)
	if !(luma >= 90 && luma <= 120) {
		s.Errorf("Luma value %d of face auto exposure is not in range [90, 120]", luma)
	}
}
