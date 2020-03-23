// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const adbSideloadingBootLockboxKey = "arc_sideloading_allowed"

func init() {
	testing.AddTest(&testing.Test{
		Func: AdbSideload,
		Desc: "Enables the Adb Sideloading flag and further checks that a warning UI is displayed at login screen",
		Contacts: []string{
			"vraheja@chromium.org",
			"victorhsieh@chromium.org",
			"arc-performance@google.com", // TODO :What should I use here ????
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"}, // TODO : Not sure about these
		SoftwareDeps: []string{"reboot", "chrome", "android_all"},
		ServiceDeps:  []string{"tast.cros.arc.AdbSideloadService", "tast.cros.example.ChromeService", "tast.cros.security.BootLockboxService"},
		Timeout:      5 * time.Minute,
	})
}

func AdbSideload(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Setting bootlock box vale to 0 to enable from UI later
	client := security.NewBootLockboxServiceClient(cl.Conn)
	client.Store(ctx, &security.StoreBootLockboxRequest{Key: adbSideloadingBootLockboxKey, Value: []byte("0")})

	// Giving the user an option to enable ADB sideloading through UI later
	service := arc.NewAdbSideloadServiceClient(cl.Conn)
	if _, err := service.EnableAdbSideloadFlag(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failing to set the Enable ADB Sideloading flag in Local State: ", err)
	}

	// Clicking OK on the Warning UI to confirm ADB sideloading
	service = arc.NewAdbSideloadServiceClient(cl.Conn)
	if _, err := service.EnableAdbConfirm(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failing to enable ADB sideloading through the UI: ", err)
	}

}
