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

type testCase struct {
	skew    *version.Version
	isValid bool // true if it's a valid supportnig skew
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateStatefulToRootfs,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that Rootfs Lacros is selected when it is newer than Stateful Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		// lacrosComponent is a runtime var to specify a name of the component which Stateful Lacros is provisioned to.
		Vars: []string{"lacrosComponent"},
		// TODO(crbug.com/1258214): Add a parameterized test for an edge case.
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
			Val: testCase{
				skew:    version.New(0, 0, 1000, 0), // 0 major -1000 build version skew from rootfs-lacros
				isValid: true,
			},
		}, {
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
			Val: testCase{
				skew:    version.New(0, 0, 1000, 0), // 0 major -1000 build version skew from rootfs-lacros
				isValid: true,
			},
		}, {
			Name:              "no_skew",
			ExtraSoftwareDeps: []string{"lacros_stable"},
			Val: testCase{
				skew:    version.New(0, 0, 0, 0), // no skew. rootfs-lacros and stateful-lacros will be the same version. rootfs-lacros should be used.
				isValid: true,
			},
		}, {
			Name:              "invalid_skew",
			ExtraSoftwareDeps: []string{"lacros_stable"},
			Val: testCase{
				skew:    version.New(10, 0, 0, 0), // invalid skew; -10 milestone older than ash-chrome. if stateful-lacros is incompatible with ash-chrome, rootfs-lacros should be used.
				isValid: false,
			},
		}},
		Timeout: 5 * time.Minute,
	})
}

func UpdateStatefulToRootfs(ctx context.Context, s *testing.State) {
	// Create a UpdateTestService client.
	conn, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	utsClient := lacrosservice.NewUpdateTestServiceClient(conn.Conn)

	// Set the version of Stateful Lacros.
	// In this test Stateful Lacros needs to be older than Rootfs Lacros, but still a valid version skew (so, Lacros should be open from Rootfs)
	rootfsLacrosVersion, err := lacrosupdate.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	ashVersion, err := lacrosupdate.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}

	statefulLacrosVersion := rootfsLacrosVersion
	skew := s.Param().(testCase).skew
	isValid := s.Param().(testCase).isValid
	statefulLacrosVersion.Decrement(skew)
	s.Logf("Versions: ash=%s rootfs-lacros=%s stateful-lacros=%s", ashVersion.GetString(), rootfsLacrosVersion.GetString(), statefulLacrosVersion.GetString())
	if !statefulLacrosVersion.IsValid() {
		s.Fatal("Invalid Stateful Lacros version: ", statefulLacrosVersion)
	} else if statefulLacrosVersion.IsNewerThan(rootfsLacrosVersion) {
		s.Fatalf("Invalid Stateful Lacros version: %v, should be older than or equal to Rootfs: %v", statefulLacrosVersion, rootfsLacrosVersion)
	} else if isValid != statefulLacrosVersion.IsSkewValid(ashVersion) {
		s.Fatalf("Invalid Stateful Lacros version: %v, not expected skew to Ash: %v, should be a valid skew? %v", statefulLacrosVersion, ashVersion, isValid)
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
