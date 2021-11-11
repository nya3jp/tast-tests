// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	"chromiumos/tast/remote/bundles/cros/lacros/update"
	lacrosupdate "chromiumos/tast/remote/bundles/cros/lacros/update"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateStatefulToRootfs,
		Desc:         "Tests that Rootfs Lacros is selected when it is newer than Stateful Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		// lacrosComponent is a runtime var to specify a name of the component which Stateful Lacros is provisioned to.
		Vars: []string{"lacrosComponent"},
		// TODO(crbug.com/1258214): Add a parameterized test for an edge case.
		Params: []testing.Param{{
			Val: version.New(0, 0, 1000, 0),
		}, {
			Name: "skew_1major",
			Val:  version.New(1, 0, 0, 0),
		}},
		Timeout: 5 * time.Minute,
	})
}

func UpdateStatefulToRootfs(ctx context.Context, s *testing.State) {
	// Create a UpdateTestService client.
	conn, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	utsClient := lacrosservice.NewUpdateTestServiceClient(conn.Conn)

	// Set the version of Stateful Lacros.
	// In this test Stateful Lacros needs to be older than Rootfs Lacros, but still a valid version skew (so, Lacros is open from Rootfs)
	rootfsLacrosVersion, err := lacrosupdate.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	ashVersion, err := lacrosupdate.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}

	statefulLacrosVersion := ashVersion
	skew := s.Param().(version.Version)
	statefulLacrosVersion.Decrement(skew.Major(), skew.Minor(), skew.Build(), skew.Patch())
	s.Log("ashVersion: ", ashVersion.GetString())
	s.Log("rootfsLacrosVersion: ", rootfsLacrosVersion.GetString())
	s.Log("statefulLacrosVersion: ", statefulLacrosVersion.GetString())
	if !statefulLacrosVersion.IsValid() {
		s.Fatal("Invalid Stateful Lacros version: ", statefulLacrosVersion)
	} else if !statefulLacrosVersion.IsOlderThan(rootfsLacrosVersion) {
		s.Fatalf("Invalid Stateful Lacros version: %v, should be older than Rootfs: %v", statefulLacrosVersion, rootfsLacrosVersion)
	} else if !statefulLacrosVersion.IsSkewValid(ashVersion) {
		// TODO(crbug.com/1258138): Check version skew once implemented in production.
		s.Fatalf("Invalid Stateful Lacros version: %v, should be compatible with supported version skews for Ash: %v", statefulLacrosVersion, ashVersion)
	}

	// Get the component to override from the runtime var. Defaults to Lacros dev channel.
	statefulLacrosComponent, err := lacrosupdate.LacrosComponentVar(s)
	if err != nil {
		s.Fatal("Failed to get Lacros component: ", err)
	}

	// Deferred cleanup to always reset to the previous state with no provisioned files.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		update.SaveLogsFromDut(ctx, s.DUT(), s.OutDir())
		if err := lacrosupdate.ClearLacrosUpdate(ctx, utsClient); err != nil {
			s.Log("Failed to clean up provisioned Lacros: ", err)
		}
	}(ctxForCleanup)

	// Simulate that an older version of Stateful Lacros has been installed than Rootfs Lacros.
	if err := lacrosupdate.ProvisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), s.DUT(), statefulLacrosVersion.GetString(), statefulLacrosComponent); err != nil {
		s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}

	// Verify that a newer version (Rootfs Lacros) is selected.
	if err := lacrosupdate.VerifyLacrosUpdate(ctx, lacrosservice.BrowserType_LACROS_ROOTFS, rootfsLacrosVersion.GetString(), "" /* no component for rootfs lacros */, utsClient); err != nil {
		s.Fatal("Failed to verify provisioned Lacros version: ", err)
	}
}
