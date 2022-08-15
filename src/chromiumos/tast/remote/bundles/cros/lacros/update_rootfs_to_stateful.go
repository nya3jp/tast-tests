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
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateRootfsToStateful,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that Stateful Lacros is selected when it is newer than Rootfs Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		// lacrosComponent is a runtime var to specify a name of the component which Lacros is provisioned to.
		Vars: []string{"lacrosComponent"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		},
		/* Disabled due to <1% pass rate over 30 days. See b/241943137
		{
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}
		*/
		},

		Timeout: 5 * time.Minute,
	})
}

func UpdateRootfsToStateful(ctx context.Context, s *testing.State) {
	// Create a UpdateTestService client.
	conn, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	utsClient := lacrosservice.NewUpdateTestServiceClient(conn.Conn)

	// Bump up the major version of Stateful Lacros to be newer than of Rootfs
	// one in order to simulate the desired test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := update.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	ashVersion, err := update.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}
	statefulLacrosVersion := rootfsLacrosVersion
	// TODO(crbug.com/1258138): Update the supported version skew policy once implemented.
	statefulLacrosVersion.Increment(version.New(9000, 0, 0, 0))
	if !statefulLacrosVersion.IsValid() {
		s.Fatal("Invalid Stateful Lacros version: ", statefulLacrosVersion)
	} else if !statefulLacrosVersion.IsNewerThan(rootfsLacrosVersion) {
		s.Fatalf("Invalid Stateful Lacros version: %v, should not be older than Rootfs: %v", statefulLacrosVersion, rootfsLacrosVersion)
	} else if !statefulLacrosVersion.IsSkewValid(ashVersion) {
		s.Fatalf("Invalid Stateful Lacros version: %v, should be compatible with Ash: %v", statefulLacrosVersion, ashVersion)
	}

	// Get the component to override from the runtime var. Defaults to Lacros dev channel.
	overrideComponent, err := update.LacrosComponentVar(s)
	if err != nil {
		s.Fatal("Failed to get Lacros component: ", err)
	}

	// Deferred cleanup to always reset to the previous state with no provisioned files.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		update.SaveLogsFromDut(ctx, s.DUT(), s.OutDir())
		if err := update.ClearLacrosUpdate(ctx, utsClient); err != nil {
			s.Log("Failed to clean up provisioned Lacros: ", err)
		}
	}(ctxForCleanup)

	// Provision Stateful Lacros from the Rootfs Lacros image file with the simulated version and component.
	if err := update.ProvisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), s.DUT(), statefulLacrosVersion.GetString(), overrideComponent); err != nil {
		s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := update.VerifyLacrosUpdate(ctx, lacrosservice.BrowserType_LACROS_STATEFUL, statefulLacrosVersion.GetString(), overrideComponent, utsClient); err != nil {
		s.Fatal("Failed to verify provisioned Lacros version: ", err)
	}
}
