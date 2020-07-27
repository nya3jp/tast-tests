// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const adbSideloadingBootLockboxKey = "arc_sideloading_allowed"

func init() {
	testing.AddTest(&testing.Test{
		Func: AdbSideloadWarningUI,
		Desc: "Enables the Adb Sideloading flag and further checks that a warning UI is displayed at login screen",
		Contacts: []string{
			"vraheja@chromium.org",
			"victorhsieh@chromium.org",
			"arc-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.AdbSideloadService", "tast.cros.example.ChromeService", "tast.cros.security.BootLockboxService"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: []string{"arc.AdbSideloadWarningUI.signinProfileTestExtensionManifestKey"},
	})
}

func AdbSideloadWarningUI(ctx context.Context, s *testing.State) {
	s.Log("Rebooting")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Performing the test twice - for confirm and cancel buttons on the UI
	s.Log("Starting to perform the test to click on the Confirm button of the UI")
	doAdbSideloadAction(ctx, s, "confirm")

	s.Log("Starting to perform the test to click on the Cancel button of the UI")
	doAdbSideloadAction(ctx, s, "cancel")
}

func resetBootlockboxValue(ctx context.Context, s *testing.State, bootLockboxClient security.BootLockboxServiceClient) {
	// Setting bootlock box value to 0 to ensure test works correctly
	if _, err := bootLockboxClient.Store(ctx, &security.StoreBootLockboxRequest{Key: adbSideloadingBootLockboxKey, Value: []byte("0")}); err != nil {
		s.Fatal("Unable to set the boot lockbox value to 0: ", err)
	}
}

func doAdbSideloadAction(ctx context.Context, s *testing.State, requestAction string) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	bootLockboxClient := security.NewBootLockboxServiceClient(cl.Conn)
	resetBootlockboxValue(ctx, s, bootLockboxClient)

	// Connecting to ADB Sideloading service
	manifestKey := s.RequiredVar("arc.AdbSideloadWarningUI.signinProfileTestExtensionManifestKey")
	signInRequest := arcpb.SigninRequest{
		Key: manifestKey,
	}
	service := arc.NewAdbSideloadServiceClient(cl.Conn)
	if _, err := service.SetRequestAdbSideloadFlag(ctx, &signInRequest); err != nil {
		s.Fatal("Failing to set the Enable ADB Sideloading flag in Local State: ", err)
	}

	// Restarting chrome to handle the request of showing the dialog. Clicking the button on the dialog to confirm or cancel ADB sideloading
	service = arc.NewAdbSideloadServiceClient(cl.Conn)
	request := arcpb.AdbSideloadServiceRequest{
		Action: requestAction,
	}
	if _, err := service.ConfirmEnablingAdbSideloading(ctx, &request); err != nil {
		s.Fatal("Failing to change ADB sideloading through the UI: ", err)
	}

	// Read and verify that bootlock box value is set to desired value
	response, err := bootLockboxClient.Read(ctx, &security.ReadBootLockboxRequest{Key: adbSideloadingBootLockboxKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	var expected []byte
	if requestAction == "confirm" {
		expected = []byte("1")
	} else if requestAction == "cancel" {
		expected = []byte("0")
	} else {
		s.Fatalf("Unrecognized Action = %s", requestAction)
	}
	s.Logf("Boot Lockbox value after button click: %s", string(response.Value))
	if !bytes.Equal(response.Value, expected) {
		s.Errorf("Actual Boot lockbox value = %s, Expected value = %s", response.Value, string(expected))
	}

	// Restoring boot lockbox value to 0
	resetBootlockboxValue(ctx, s, bootLockboxClient)
}
