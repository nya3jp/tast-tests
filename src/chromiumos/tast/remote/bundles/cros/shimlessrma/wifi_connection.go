// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

const (
	offlineOperationTimeout = 2 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiConnection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test wifi connection will be forgotten after Shimless RMA",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.shimlessrma.AppService",
		},
		Fixture: fixture.NormalMode,
		Timeout: 10 * time.Minute,
	})
}

func WifiConnection(ctx context.Context, s *testing.State) {
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

	if err := uiHelper.PrepareOfflineTest(ctx); err != nil {
		s.Fatal("Fail to prepare offline test: ", err)
	}
	s.Log("Prepare offline test successfully")

	// Limit the timeout for the preparation steps.
	offlineCtx, cancel := context.WithTimeout(ctx, offlineOperationTimeout)
	defer cancel()

	// Ignore any error in the following gRPC call,
	// because ethernet is turned off in that call.
	// As a result, we always get gRPC connection error.
	_ = uiHelper.WelcomeAndNetworkPageOperationOffline(offlineCtx)

	s.Log("Offline time is completed")

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}

	if err := uiHelper.VerifyOfflineOperationSuccess(ctx); err != nil {
		s.Fatal("Offline operation failed: ", err)
	}
	s.Log("offline operation succeed")

	if err := uiHelper.VerifyWifiIsForgotten(ctx); err != nil {
		s.Fatal("Failed to forget wifi: ", err)
	}
}
