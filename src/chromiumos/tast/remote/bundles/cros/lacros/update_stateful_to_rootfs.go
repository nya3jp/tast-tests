// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	lacrosupdate "chromiumos/tast/remote/bundles/cros/lacros/update"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/lacros"
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
		// lacrosComponent is a runtime var to specify a name of the component which Lacros is provisioned to.
		Vars:    []string{"lacrosComponent"},
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

	// Bump down the major version of Stateful Lacros to be newer than of Rootfs
	// one in order to simulate the desired test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := lacrosupdate.GetRootfsLacrosVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	// TDOO: Use ashVersion
	ashVersion, err := lacrosupdate.GetAshVersion(ctx, s.DUT(), utsClient)
	if err != nil {
		s.Fatal("Failed to get the Ash version: ", err)
	}

	statefulLacrosVersion := ashVersion
	// TODO(crbug.com/1258138): Use a supported version skew.
	// TODO: This will push 2 milestone older version of Ash, resulting in getting out of supported version skew.
	statefulLacrosVersion.Decrement(2, 0, 0, 0)
	s.Log("rootfsLacrosVersion: ", rootfsLacrosVersion)
	s.Log("statefulLacrosVersion: ", statefulLacrosVersion)
	s.Log("ashVersion: ", ashVersion)
	if !statefulLacrosVersion.IsValid() {
		s.Fatal("Invalid Stateful Lacros version: ", statefulLacrosVersion)
	}
	// TODO: Commented to bypass version checks for simulating error condition.
	// } else if !statefulLacrosVersion.IsOlderThan(ashVersion) {
	// s.Fatal("Invalid Stateful Lacros version, should be older than Ash: ", statefulLacrosVersion)
	// } else if !statefulLacrosVersion.IsSkewValid(ashVersion) {
	// 	s.Fatal("Invalid Stateful Lacros version, should be compatible with supported version skews: ", statefulLacrosVersion)

	// Get the component to override from the runtime var. Defaults to Lacros dev channel.
	overrideComponent, err := lacrosupdate.LacrosComponentVar(s)
	if err != nil {
		s.Fatal("Failed to get Lacros component: ", err)
	}

	// Deferred cleanup to always reset to the previous state with no provisioned files.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := lacrosupdate.ClearLacrosUpdate(ctx, utsClient); err != nil {
			s.Log("Failed to clean up provisioned Lacros: ", err)
		}
	}(ctxForCleanup)

	// Provision Stateful Lacros from the Rootfs Lacros image file with the simulated version and component.
	if err := lacrosupdate.ProvisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), s.DUT(), statefulLacrosVersion.GetString(), overrideComponent); err != nil {
		s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := verifyLacrosUpdate2(ctx, rootfsLacrosVersion.GetString(), overrideComponent, utsClient); err != nil {
		s.Fatal("Failed to verify provisioned Lacros version: ", err)
	}
}

// verifyLacrosUpdate2 calls a RPC to the test service to verify the provisioned Lacros update is installed and selected in runtime on a DUT as expected.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func verifyLacrosUpdate2(ctx context.Context, overrideVersion, overrideComponent string, utsClient lacros.UpdateTestServiceClient) error {
	// Build browser contexts for a test request.
	ashCtx := &lacrosservice.BrowserContext{
		Browser: lacrosservice.BrowserType_ASH,
		Opts: []string{
			"--enable-features=LacrosSupport",
			"--component-updater=url-source=" + lacroscommon.BogusComponentUpdaterURL, // Block Component Updater.
			// TODO:
			// "--lacros-stability=less-stable",
		},
	}
	lacrosCtx := &lacrosservice.BrowserContext{
		Browser: lacrosservice.BrowserType_LACROS_STATEFUL,
	}

	// Send a test request to the DUT.
	res, err := utsClient.VerifyUpdate(ctx,
		&lacrosservice.VerifyUpdateRequest{
			AshContext:               ashCtx,
			ProvisionedLacrosContext: []*lacrosservice.BrowserContext{lacrosCtx},
			ExpectedBrowser:          lacrosservice.BrowserType_LACROS_ROOTFS,
			ExpectedVersion:          overrideVersion,
			ExpectedComponent:        overrideComponent,
			UseUi:                    true,
		})
	if err != nil {
		return errors.Wrap(err, "verifyLacrosUpdate: failed to verify version on Lacros")
	}
	if res.Result.Status != lacrosservice.TestResult_PASSED {
		return errors.Wrapf(err, "verifyLacrosUpdate: returns test failure status: %v", res.Result)
	}
	return nil
}
