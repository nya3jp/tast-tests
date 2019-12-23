// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	//"chromiumos/tast/local/chrome"
	//"chromiumos/tast/local/upstart"
	//"chromiumos/tast/local/testexec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"

	//"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
	//"encoding/hex"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AdbSideload,
		Desc: "Signs in to DUT and measures Android boot performance metrics",
		Contacts: []string{
			"cywang@chromium.org", // Original author.
			"niwa@chromium.org",   // Tast port author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"reboot", "chrome", "android_all"},
		ServiceDeps:  []string{"tast.cros.arc.AdbSideloadService", "tast.cros.example.ChromeService"},
		Timeout:      5 * time.Minute,
		//Pre:          arc.Booted(),
	})
}

func AdbSideload(ctx context.Context, s *testing.State) {

	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	/*
		// Reboot to make boot lockbox writable
		s.Log("Rebooting")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		stopUI, err := s.DUT().Command("stop", "ui").Output(ctx)
		if err != nil {
			s.Log("Failed to stop the UI remotely")
		}
		s.Log("Stop UI Command Output = ", string(stopUI))
	*/

	s.Log("Starting the service")
	service := arc.NewAdbSideloadServiceClient(cl.Conn)

	if _, err := service.EnableAdbSideloadFlag(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failing to set the PREFS: ", err)
	}

	/*
		s.Log("Trying to start the UI")
		//Actually reboot
		if err = upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Chrome logout failed: ", err)
		}
	*/

	/*
		s.Log("Rebooting")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)
	*/

	time.Sleep(10 * time.Second)
	service = arc.NewAdbSideloadServiceClient(cl.Conn)

	if _, err := service.EnableAdbConfirm(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failing to set the PREFS: ", err)
	}

}
