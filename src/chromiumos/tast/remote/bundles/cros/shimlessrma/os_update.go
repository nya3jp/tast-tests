// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

const (
	preUpdateTimeoutN2N  = 1 * time.Minute
	postUpdateTimeoutN2N = 2 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OSUpdate,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test OS Update in Shimless RMA",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"reboot", "chrome", "auto_update_stable"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.shimlessrma.AppService",
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Fixture: fixture.NormalMode,
		Timeout: 10 * time.Minute,
	})
}

func OSUpdate(ctx context.Context, s *testing.State) {
	// Limit the timeout for the preparation steps.
	preCtx, cancel := context.WithTimeout(ctx, preUpdateTimeoutN2N)
	defer cancel()

	lsbContent := map[string]string{
		lsbrelease.Version:     "",
		lsbrelease.BuilderPath: "",
	}

	err := updateutil.FillFromLSBRelease(preCtx, s.DUT(), s.RPCHint(), lsbContent)
	if err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	// Original image version to compare it with the version after the update.
	originalVersion := lsbContent[lsbrelease.Version]
	// Builder path is used in selecting the update image.
	builderPath := lsbContent[lsbrelease.BuilderPath]

	s.Logf("Orignal version is %s and builder path is %s", originalVersion, builderPath)

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	dut := firmwareHelper.DUT
	key := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Fail to init servo: ", err)
	}

	uiHelper, err := rmaweb.NewUIHelper(preCtx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}

	osUpdateAction := action.Combine("verify it hits OS update page",
		uiHelper.WelcomePageOperation,
		uiHelper.OsUpdateOperation,
	)

	// Setup Mock Auto Update Server.
	if err := updateutil.UpdateFromGSByCustomLSB(ctx, s.DUT(), s.OutDir(), s.RPCHint(), builderPath, osUpdateAction); err != nil {
		s.Fatalf("Failed to update DUT to image for %q from GS: %v", builderPath, err)
	}
}
