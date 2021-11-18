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
		Func:         UpdateStatefulToStateful,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests that the newest Stateful Lacros is selected when there are more than one Stateful Lacros installed. This can also test version skew policy in Ash by provisioning any major version skews",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		// lacrosComponent is a runtime var to specify a name of the component which Lacros is provisioned to.
		Vars: []string{"lacrosComponent"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func UpdateStatefulToStateful(ctx context.Context, s *testing.State) {
	// Create a UpdateTestService client.
	conn, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	utsClient := lacrosservice.NewUpdateTestServiceClient(conn.Conn)

	// The versions of Stateful Lacros.
	// Used to verify the update path of Stateful => Stateful in ascending order on the same channel.
	// Each version should be newer than Rootfs Lacros, but not over the maximum version skew of ({Ash or Rootfs} + 2 major).
	rootfsLacrosVersion, err := update.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	ashVersion, err := update.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}
	baseVersion := rootfsLacrosVersion
	// TODO(crbug.com/1258138): Use a supported version skew, instead of +9000 major.
	statefulLacrosVersions := []version.Version{
		baseVersion.Increment(0, 1, 0, 0),    // +0 major +1 minor
		baseVersion.Increment(1, 0, 0, 0),    // +1 major
		baseVersion.Increment(9000, 0, 0, 0), // +9000 major
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

	// Verify the update from Stateful => Stateful.
	for _, overrideVersion := range statefulLacrosVersions {
		// TODO(hyungtaekim): Consider a helper function to check versions for all tests.
		if !overrideVersion.IsValid() {
			s.Fatal("Invalid Stateful Lacros version: ", overrideVersion)
		} else if !overrideVersion.IsNewerThan(rootfsLacrosVersion) {
			s.Fatalf("Invalid Stateful Lacros version: %v, should not be older than Rootfs: %v", overrideVersion, rootfsLacrosVersion)
		} else if !overrideVersion.IsSkewValid(ashVersion) {
			s.Fatalf("Invalid Stateful Lacros version: %v, should be compatible with Ash: %v", overrideVersion, ashVersion)
		}

		// Provision Stateful Lacros from the Rootfs Lacros image file with the simulated version and component.
		if err := update.ProvisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), s.DUT(), overrideVersion.GetString(), overrideComponent); err != nil {
			s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
		}

		// Verify that the expected Stateful Lacros version/component is selected.
		if err := update.VerifyLacrosUpdate(ctx, lacrosservice.BrowserType_LACROS_STATEFUL, overrideVersion.GetString(), overrideComponent, utsClient); err != nil {
			s.Fatal("Failed to verify provisioned Lacros version: ", err)
		}
	}
}
