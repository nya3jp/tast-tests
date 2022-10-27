// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type sensor struct {
	name          string
	stateFilePath string
}

var sensorAccel = sensor{
	name:          "accel",
	stateFilePath: "state_files/lid_accel_calibration.json",
}

var sensorGyro = sensor{
	name:          "gyro",
	stateFilePath: "state_files/base_gyro_calibration.json",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Calibration,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test accelerometer/gyro calibration in Shimless RMA",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		Data: []string{sensorAccel.stateFilePath, sensorGyro.stateFilePath},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.shimlessrma.AppService",
		},
		Fixture: fixture.NormalMode,
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "accel",
			Val:  sensorAccel,
		}, {
			Name: "gyro",
			Val:  sensorGyro,
		}},
	})
}

func Calibration(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	dut := firmwareHelper.DUT
	key := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")

	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Fail to init servo: ", err)
	}

	uiHelper, err := rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}

	component := s.Param().(sensor)
	statePath := s.DataPath(component.stateFilePath)

	s.Logf("The path of state file is: %s", statePath)

	b, err := os.ReadFile(statePath)
	if err != nil {
		s.Fatal("Fail to read state file on host: ", err)
	}

	stateFileContent := string(b)
	if err := uiHelper.OverrideStateFile(ctx, stateFileContent); err != nil {
		s.Fatal("Fail to override state file: ", err)
	}

	// Wait for reboot start.
	if err := testing.Sleep(ctx, rmaweb.WaitForRebootStart); err != nil {
		s.Fatal("Fail to sleep to wait for reboot start: ", err)
	}

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

	if component == sensorAccel {
		if err := uiHelper.CalibrateLidAccelerometerPageOperation(ctx); err != nil {
			s.Fatal("Fail to calibrate lid accelerometer: ", err)
		}
	} else {
		if err := uiHelper.CalibrateBaseGyroPageOperation(ctx); err != nil {
			s.Fatal("Fail to calibrate base gyro: ", err)
		}
	}
}
