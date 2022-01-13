// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestFixture,
		Desc:         "Test fixture",
		Contacts:     []string{"jinrongwu@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "buster_stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Fixture:           "crostiniBuster",
				Timeout:           7 * time.Minute,
			},
		},
	})
}

// TestFixture is a test to test the fixture, not a real test.
// TODO (jinrongwu): to remove it once all crostini test cases have been migrated to fixture successfully.
func TestFixture(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(crostini.FixtureData).Tconn
	kb := s.FixtValue().(crostini.FixtureData).KB

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, crostini.PostTimeout)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	if err := filesApp.OpenLinuxFiles()(ctx); err != nil {
		s.Fatal("Failed to open Linux files: ", err)
	}

	// Open Terminal.
	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	defer terminalApp.Exit(kb)(cleanupCtx)

	s.Log("=================================================")
	s.Log("SUCCEEDED")
	s.Log("=================================================")
}
