// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
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
			"arc-core@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"}, // TODO : Not sure about these
		SoftwareDeps: []string{"reboot", "chrome", "android_all"},
		ServiceDeps:  []string{"tast.cros.arc.AdbSideloadService", "tast.cros.example.ChromeService", "tast.cros.security.BootLockboxService"},
		Timeout:      5 * time.Minute,
	})
}

func AdbSideload(ctx context.Context, s *testing.State) {

	// Performing the test twice - for confirm and cancel buttons on the UI
	s.Log("Starting to perform the test to click on the Confirm button of the UI")
	doAdbSideloadAction(ctx, s, "confirm")

	s.Log("Starting to perform the test to click on the Cancel button of the UI")
	doAdbSideloadAction(ctx, s, "cancel")
}

func resetBootlockboxValue(ctx context.Context, cl *rpc.Client) {

	// Setting bootlock box value to 0 to ensure test works correctly
	client := security.NewBootLockboxServiceClient(cl.Conn)
	client.Store(ctx, &security.StoreBootLockboxRequest{Key: adbSideloadingBootLockboxKey, Value: []byte("0")})
}

func doAdbSideloadAction(ctx context.Context, s *testing.State, requestAction string) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	resetBootlockboxValue(ctx, cl)

	// Connecting to ADB Sideloading service
	service := arc.NewAdbSideloadServiceClient(cl.Conn)
	if _, err := service.SetRequestAdbSideloadFlag(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failing to set the Enable ADB Sideloading flag in Local State: ", err)
	}

	// Clicking the button on the dialog to confirm or cancel ADB sideloading
	service = arc.NewAdbSideloadServiceClient(cl.Conn)
	request := arcpb.AdbSideloadServiceRequest{
		Action: requestAction,
	}
	if _, err := service.ConfirmEnablingAdbSideloading(ctx, &request); err != nil {
		s.Fatal("Failing to change ADB sideloading through the UI: ", err)
	}

	// Read and verify that bootlock box value is set to desired value
	bootLockboxClient := security.NewBootLockboxServiceClient(cl.Conn)
	response, err := bootLockboxClient.Read(ctx, &security.ReadBootLockboxRequest{Key: adbSideloadingBootLockboxKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	expectedByte := []byte("")
	if requestAction == "confirm" {
		expectedByte = []byte("1")
	} else if requestAction == "cancel" {
		expectedByte = []byte("0")
	} else {
		s.Fatalf("Unrecognized Action = %s", requestAction)
	}
	s.Logf("Boot Lockbox value after button click: %s", string(response.Value))
	if bytes.Equal(response.Value, expectedByte) == false {
		s.Fatalf("Actual Boot lockbox value = %s, Expected value = %s", response.Value, string(expectedByte))
	}

	// Restoring boot lockbox value to 0
	resetBootlockboxValue(ctx, cl)
}
