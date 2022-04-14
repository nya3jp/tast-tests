// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"

	"github.com/godbus/dbus"

	pb "chromiumos/system_api/hps_proto"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DBus,
		Desc: "Check that hpsd can be connected to via dbus",
		Contacts: []string{
			"evanbenn@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(b/227525135): re-enable when we have some brya DUTs with HPS
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("brya")),
		SoftwareDeps: []string{"hps"},
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
}
