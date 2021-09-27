// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"time"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/lacros"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateRootfsToStateful,
		Desc:         "Tests that Stateful Lacros is selected when it is newer than Rootfs Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		// lacros.UpdateRootfsToStateful.component is a runtime var to specify a name of the component which Lacros is provisioned to.
		Vars:    []string{"lacros.UpdateRootfsToStateful.component"},
		Timeout: 5 * time.Minute,
	})
}

func UpdateRootfsToStateful(ctx context.Context, s *testing.State) {
	// Set up a RPC client to the update test service on a DUT.
	dut := s.DUT()
	conn, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	utsClient := lacrosservice.NewUpdateTestServiceClient(conn.Conn)

	// Bump up the major version of Stateful Lacros to be newer than of Rootfs
	// one in order to simulate the desired test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := getRootfsLacrosVersion(ctx, dut, utsClient)
	if err != nil {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	} else if !rootfsLacrosVersion.IsValid() {
		s.Fatal("Failed to get the Rootfs Lacros version, invalid version")
	}
	statefulLacrosVersion := rootfsLacrosVersion
	// TODO: Stay within supported version skew range of [0, +2] milestone from Ash, not +9000.
	statefulLacrosVersion.Increment(9000, 0, 0, 0)

	// Override the component if necessary for debugging. Defaults to Lacros dev channel.
	overrideComponent := "lacros-dogfood-dev"
	if componentVar, ok := s.Var("lacros.UpdateRootfsToStateful.component"); ok {
		overrideComponent = componentVar
	}

	// Deferred cleanup to always reset to the previous state with no provisioned files.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := clearLacrosUpdate(ctx, utsClient); err != nil {
			s.Log("Failed to clean up provisioned Lacros: ", err)
		}
	}(ctxForCleanup)

	// Provision Stateful Lacros from the Rootfs Lacros image file with the simulated version and component.
	if err := provisionLacrosFromRootfsLacrosImagePath(ctx, provision.TLSAddrVar.Value(), dut, statefulLacrosVersion.GetString(), overrideComponent); err != nil {
		s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := verifyLacrosUpdate(ctx, statefulLacrosVersion.GetString(), overrideComponent, utsClient); err != nil {
		s.Fatal("Failed to verify provisioned Lacros version: ", err)
	}
}

// getRootfsLacrosVersion returns the version of Lacros Chrome in the Rootfs partition.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func getRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, utsClient lacrosservice.UpdateTestServiceClient) (version.Version, error) {
	req := &lacrosservice.GetBrowserVersionRequest{
		Browser: lacrosservice.BrowserType_LACROS_ROOTFS,
	}
	res, err := utsClient.GetBrowserVersion(ctx, req)
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}
	// Note that there must be only one Rootfs Lacros version on a DUT.
	return version.Parse(res.Versions[0]), nil
}

// provisionLacrosFromRootfsLacrosImagePath calls a RPC to the TLS to provision Stateful Lacros from the source of Rootfs Lacros image installation path.
// This is useful when making the test hermetic by removing external source of Lacros binary.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func provisionLacrosFromRootfsLacrosImagePath(ctx context.Context, tlsAddr string, dut *dut.DUT, overrideVersion, overrideComponent string) error {
	tlsClient, err := provision.Dial(ctx, tlsAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to TLS")
	}
	defer tlsClient.Close()

	dutName := dut.HostName()
	testing.ContextLogf(ctx, "Provisioning Lacros from Rootfs image: DUT=%v, overrideVersion=%v, overrideComponent=%v", dutName, overrideVersion, overrideComponent)
	return tlsClient.ProvisionLacrosFromDeviceFilePath(
		ctx, dutName,
		lacroscommon.RootfsLacrosImageFileURL, overrideVersion, filepath.Join(lacroscommon.LacrosRootComponentPath, overrideComponent))
}

// verifyLacrosUpdate calls a RPC to the test service to verify the provisioned Lacros update is installed and selected in runtime on a DUT as expected.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func verifyLacrosUpdate(ctx context.Context, overrideVersion, overrideComponent string, utsClient lacros.UpdateTestServiceClient) error {
	// Build browser contexts for a test request.
	ashCtx := &lacrosservice.BrowserContext{
		Browser: lacrosservice.BrowserType_ASH,
		Opts: []string{
			"--enable-features=LacrosSupport",
			"--component-updater=url-source=" + lacroscommon.BogusComponentUpdaterURL, // Block Component Updater.
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
			ExpectedBrowser:          lacrosservice.BrowserType_LACROS_STATEFUL,
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

// clearLacrosUpdate calls a RPC to the test service to remove provisioned Lacros and reset to the previous state.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func clearLacrosUpdate(ctx context.Context, utsClient lacrosservice.UpdateTestServiceClient) error {
	if _, err := utsClient.ClearUpdate(ctx, &lacrosservice.ClearUpdateRequest{}); err != nil {
		return errors.Wrap(err, "clearLacrosUpdate: failed to clear provisioned Lacros")
	}
	return nil
}
