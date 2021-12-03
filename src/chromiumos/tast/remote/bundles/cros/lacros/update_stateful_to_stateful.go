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
	skew    version.Version // version skew from rootfs-lacros
}

// Update path on the same lacros (dev-)channel. Provision will go in order from top to bottom.
var pathUpdateOnSameChannel = []updatePath{
	{lacroscommon.LacrosDevComponent, version.New(0, 1, 0, 0)}, // +0 major +1 minor from rootfs-lacros
	{lacroscommon.LacrosDevComponent, version.New(1, 0, 0, 0)}, // +1 major +0 minor
	{lacroscommon.LacrosDevComponent, version.New(2, 0, 0, 0)}, // +2 major
}

// Upgrade path when switching from dev to canary.
var pathUpgradeChannel = []updatePath{
	{lacroscommon.LacrosDevComponent, version.New(0, 1, 0, 0)},    // +0 major +1 minor on dev-channel
	{lacroscommon.LacrosCanaryComponent, version.New(1, 0, 0, 0)}, // +1 major +0 minor on canary-channel
}

// Downgrade path when switching from canary to dev.
var pathDowngradeChannel = []updatePath{
	{lacroscommon.LacrosCanaryComponent, version.New(1, 0, 0, 0)}, // +1 major +0 minor on canary-channel
	{lacroscommon.LacrosDevComponent, version.New(0, 1, 0, 0)},    // +0 major +1 minor on dev-channel
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateStatefulToStateful,
		Desc:         "Tests that a newer Stateful Lacros is selected when there are more than one Stateful Lacros installed on the same channel. Selection is per channel, thus versions on inactive lacros channels are ignored",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
			Val:               pathUpdateOnSameChannel,
		}, {
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
			Val:               pathUpdateOnSameChannel,
		}, {
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
		statefulLacrosVersion := baseVersion.Increment(updateInfo.skew.Major(), updateInfo.skew.Minor(), updateInfo.skew.Build(), updateInfo.skew.Patch())
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
