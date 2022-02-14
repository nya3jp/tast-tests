// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	//"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestFixture2,
		Desc:         "Test fixture, this should be run together with TestFixture to test PreTest and PostTest",
		LacrosStatus: testing.LacrosVariantExists,
		Contacts:     []string{"jinrongwu@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "buster_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           2 * time.Minute,
			},
			{
				Name:              "bullseye_stable",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseye",
				Timeout:           2 * time.Minute,
			},
			{
				Name:              "buster_stable_gaia",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBusterGaia",
				Timeout:           2 * time.Minute,
			},
			{
				Name:              "bullseye_stable_gaia",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseyeGaia",
				Timeout:           2 * time.Minute,
			},
			{
				Name:              "large_container",
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseyeLargeContainer",
				Timeout:           2 * time.Minute,
			},
			{
				Name:              "lacros",
				ExtraSoftwareDeps: []string{"dlc", "lacros"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBullseyeWithLacros",
				Timeout:           2 * time.Minute,
				//Val:               browser.TypeLacros,
			},
		},
	})
}

// TestFixture2 is a test to test the fixture, not a real test.
// TODO (jinrongwu): to remove it once all crostini test cases have been migrated to fixture successfully.
func TestFixture2(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(crostini.FixtureData).Tconn

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := filesApp.OpenLinuxFiles()(ctx); err != nil {
		s.Fatal("Failed to open Linux files: ", err)
	}

	// Open Terminal app.
	_, err = terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	s.Log("=================================================")
	s.Log("SUCCEEDED")
	s.Log("=================================================")
}
