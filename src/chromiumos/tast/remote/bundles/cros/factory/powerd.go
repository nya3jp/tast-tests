// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/factory/fixture"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Powerd,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test powerd is working and is in factory mode with factory toolkit",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		SoftwareDeps: append([]string{"factory_flow"}, fixture.EnsureToolkitSoftwareDeps...),
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      time.Minute,
		Fixture:      fixture.EnsureToolkit,
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
	})
}

func Powerd(ctx context.Context, s *testing.State) {
	conn, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer conn.Close(ctx)

	checkPowerdRunning(ctx, s, conn.Conn)
	checkIsFactoryMode(ctx, s, conn.Conn)
}

func checkPowerdRunning(ctx context.Context, s *testing.State, conn *grpc.ClientConn) {
	client := platform.NewUpstartServiceClient(conn)
	_, err := client.CheckJob(ctx, &platform.CheckJobRequest{JobName: "powerd"})
	if err != nil {
		s.Fatal("powerd is not running: ", err)
	}
}

func checkIsFactoryMode(ctx context.Context, s *testing.State, conn *grpc.ClientConn) {
	client := dutfs.NewClient(conn)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rawContent, err := client.ReadFile(ctx, "/var/log/power_manager/powerd.LATEST")
		if err != nil {
			return errors.Wrap(err, "cannot get powerd log")
		}
		if strings.Contains(string(rawContent), "Factory mode enabled") {
			return nil
		}
		return errors.New("cannot find log indicating is in factory mode")
	}, nil); err != nil {
		s.Fatal("DUT not in factory mode: ", err)
	}
}
