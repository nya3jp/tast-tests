// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	"chromiumos/tast/remote/bundles/cros/lacros/update"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

type updatePath struct {
	channel string
	skew    *version.Version // version skew from rootfs-lacros
}

// Test scenarios that represent different update paths to be tested in the parameterized tests.
var (
	// 1. Updates on the same channel that go with +1 minor version bump to +1 major, and to +2 major from rootfs-lacros.
	pathUpdateOnSameChannel = []updatePath{
		{
			channel: lacroscommon.LacrosDevComponent,
			skew:    version.New(0, 1, 0, 0), // +0 major +1 minor from rootfs-lacros
		},
		{
			channel: lacroscommon.LacrosDevComponent,
			skew:    version.New(1, 0, 0, 0), // +1 major +0 minor
		},
		{
			channel: lacroscommon.LacrosDevComponent,
			skew:    version.New(2, 0, 0, 0), // +2 major
		},
	}
	// 2. Upgrade to a channel of a newer milestone (eg, dev to canary) assuming that canary is one milestone ahead of dev.
	pathUpgradeChannel = []updatePath{
		{
			channel: lacroscommon.LacrosDevComponent,
			skew:    version.New(0, 1, 0, 0), // +0 major +1 minor on dev-channel
		},
		{
			channel: lacroscommon.LacrosCanaryComponent,
			skew:    version.New(1, 0, 0, 0), // +1 major +0 minor on canary-channel
		},
	}
	// 3. Downgrade to a channel of an older milestone (eg, canary to dev)
	pathDowngradeChannel = []updatePath{
		{
			channel: lacroscommon.LacrosCanaryComponent,
			skew:    version.New(1, 0, 0, 0), // +1 major +0 minor on canary-channel
		},
		{
			channel: lacroscommon.LacrosDevComponent,
			skew:    version.New(0, 1, 0, 0), // +0 major +1 minor on dev-channel
		},
	}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateStatefulToStateful,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that the newest Stateful Lacros is selected when there are more than one Stateful Lacros installed. This can also test version skew policy in Ash by provisioning any major version skews",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
			Val:               pathUpdateOnSameChannel,
		},
			/* Disabled due to <1% pass rate over 30 days. See b/241943137
			{
				Name:              "unstable",
				ExtraSoftwareDeps: []string{"lacros_unstable"},
				Val:               pathUpdateOnSameChannel,
			},
			*/
			{

				Name:              "channel_upgrade",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val:               pathUpgradeChannel,
			}, {
				Name:              "channel_downgrade",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val:               pathDowngradeChannel,
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
	// Used to verify the update path of Stateful => Stateful (1) on the same channel and (2) when switching channels.
	// Each version should be newer than Rootfs Lacros, but not over the maximum version skew of (Ash + 2 major).
	rootfsLacrosVersion, err := update.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	ashVersion, err := update.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}
	baseVersion := rootfsLacrosVersion

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

	// Verify the updates from Stateful => Stateful.
	for _, updateInfo := range s.Param().([]updatePath) {
		statefulLacrosVersion := baseVersion.Increment(updateInfo.skew)
		overrideComponent := updateInfo.channel

		// TODO(hyungtaekim): Consider a helper function to check versions for all tests.
		if !statefulLacrosVersion.IsValid() {
			s.Fatal("Invalid Stateful Lacros version: ", statefulLacrosVersion)
		} else if !statefulLacrosVersion.IsNewerThan(rootfsLacrosVersion) {
			s.Fatalf("Invalid Stateful Lacros version: %v, should not be older than Rootfs: %v", statefulLacrosVersion, rootfsLacrosVersion)
		} else if !statefulLacrosVersion.IsSkewValid(ashVersion) {
			s.Fatalf("Invalid Stateful Lacros version: %v, should be compatible with Ash: %v", statefulLacrosVersion, ashVersion)
		}

		// Provision Stateful Lacros from the Rootfs Lacros image file with the simulated version and component.
		if err := update.ProvisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), s.DUT(), statefulLacrosVersion.GetString(), overrideComponent); err != nil {
			s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
		}

		// Verify that the expected Stateful Lacros version/component is selected.
		if err := update.VerifyLacrosUpdate(ctx, lacrosservice.BrowserType_LACROS_STATEFUL, statefulLacrosVersion.GetString(), overrideComponent, utsClient); err != nil {
			s.Fatal("Failed to verify provisioned Lacros version: ", err)
		}
	}
}
