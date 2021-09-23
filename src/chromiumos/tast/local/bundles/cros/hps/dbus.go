// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	pb "chromiumos/system_api/hps_proto"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DBus,
		Desc: "Check that hpsd can be connected to via dbus",
		Contacts: []string{
			"evanbenn@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"hps"},
	})
}

func DBus(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.Hps"
		dbusPath      = "/org/chromium/Hps"
		dbusInterface = "org.chromium.Hps"
		dbusMethod    = "EnableFeatureHpsSense"

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

	s.Log("Running EnableFeatureHpsNotify(BasicFilter) on hpsd")
	config := &pb.FeatureConfig{
		FilterConfig: &pb.FeatureConfig_BasicFilterConfig_{
			BasicFilterConfig: &pb.FeatureConfig_BasicFilterConfig{},
		},
	}
	marshalled, err := proto.Marshal(config)
	if err != nil {
		s.Fatal("Failed to marshal FeatureConfig: ", err)
	}
	if err := obj.CallWithContext(ctx, dbusInterface+"."+dbusMethod, 0, marshalled).Store(); err != nil {
		s.Error("EnableFeatureHpsNotify(BasicFilter) failed: ", err)
	} else {
		s.Log("EnableFeatureHpsNotify(BasicFilter) success")
	}
}
