// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

const (
	offlineOperationTimeout = 2 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OSUpdate,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test OS Update (as wifi connection) in Shimless RMA",
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

func OSUpdate(ctx context.Context, s *testing.State) {
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

	// Limit the timeout for the preparation steps.
	offlineCtx := ctx
	ctx, cancel := ctxutil.Shorten(offlineCtx, offlineOperationTimeout)
	defer cancel()

	// Ignore any error in the following gRPC call,
	// because ethernet is turned off in that call.
	// As a result, we always get gRPC connection error.
	_ = uiHelper.WelcomeAndNetworkPageOperationOffline(ctx)

	s.Log("Wait for 5 seconds to wait for reboot starting")
	// Wait for reboot starting.
	testing.Sleep(ctx, 5*time.Second)
	uiHelper, err = rmaweb.NewUIHelper(offlineCtx, dut, firmwareHelper, s.RPCHint(), key, false)
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
