// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"

	"github.com/godbus/dbus/v5"

	pb "chromiumos/system_api/hps_proto"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParam struct {
	CheckFirmwareVersion bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DBus,
		Desc: "Check that hpsd can be connected to via dbus",
		Contacts: []string{
			"evanbenn@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"hps"},
		Params: []testing.Param{{
			Val: testParam{
				CheckFirmwareVersion: true,
			},
			ExtraHardwareDeps: hwdep.D(hwdep.HPS()),
		}, {
			// On *-generic images, hpsd is configured to use
			// a fake peripheral which works for testing the DBus
			// interface in a VM.
			Name: "fake",
			Val: testParam{
				// It doesn't make sense to check the running firmware version
				// when we are testing against the fake peripheral.
				CheckFirmwareVersion: false,
			},
			ExtraSoftwareDeps: []string{"fake_hps"},
		}},
	})
}

func DBus(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.Hps"
		dbusPath      = "/org/chromium/Hps"
		dbusInterface = "org.chromium.Hps"
		dbusMethod    = "EnableHpsSense"

		job = "hpsd"
	)

	param := s.Param().(testParam)

	s.Logf("Restarting %s job and waiting for %s service", job, dbusName)
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to start %s: %v", job, err)
	}
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Log("Running EnableHpsSense(BasicFilter) on hpsd")
	config := &pb.FeatureConfig{
		FilterConfig: &pb.FeatureConfig_BasicFilterConfig_{
			BasicFilterConfig: &pb.FeatureConfig_BasicFilterConfig{},
		},
	}
	if err := dbusutil.CallProtoMethod(ctx, obj, dbusInterface+"."+dbusMethod, config, nil); err != nil {
		s.Error("EnableHpsSense(BasicFilter) failed: ", err)
	} else {
		s.Log("EnableHpsSense(BasicFilter) success")
	}

	hctx, err := hpsutil.NewHpsContext(ctx, "", hpsutil.DeviceTypeBuiltin, s.OutDir(), s.DUT().Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Check that HPS is running the expected firmware version.
	if param.CheckFirmwareVersion {
		runningVersion, err := hpsutil.FetchRunningFirmwareVersion(hctx)
		if err != nil {
			s.Error("Error reading running firmware version: ", err)
		}
		expectedVersion, err := hpsutil.FetchFirmwareVersionFromImage(hctx, hpsutil.FirmwarePath)
		if err != nil {
			s.Error("Error reading firmware version from image: ", err)
		}
		if runningVersion != expectedVersion {
			s.Errorf("HPS reports running firmware version %v but expected %v", runningVersion, expectedVersion)
		}
	}
}
