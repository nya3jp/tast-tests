// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/lacros"
	proto "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

const (
	statefulLacrosRootComponentPath = "/home/chronos/cros-components/"
	statefulLacrosComponent         = "lacros-dogfood-dev"
	rootfsLacrosImageFileURL        = "file:///opt/google/lacros"
	bogusComponentUpdaterURL        = "http://localhost:12345"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateRootfsToStateful,
		Desc:         "Tests that Stateful Lacros is selected when it is newer than Rootfs Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.UpdateTestService"},
		Timeout:      10 * time.Minute,
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
	utsClient := proto.NewUpdateTestServiceClient(conn.Conn)

	// Bump up the major version of Stateful Lacros to be newer than of Rootfs
	// one in order to simulate the desired test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := getRootfsLacrosVersion(ctx, dut, utsClient)
	if err != nil || !rootfsLacrosVersion.IsValid() {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	statefulLacrosVersion := rootfsLacrosVersion
	// TODO: Stay within supported version skew range of [0, +2] milestone from Ash, not +9000.
	statefulLacrosVersion.Increment(9000, 0, 0, 0)

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
	if err := provisionLacrosFromRootfsImage(ctx, provision.TLSAddrVar.Value(), dut, statefulLacrosVersion.GetString(), statefulLacrosComponent); err != nil {
		s.Fatal("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := verifyLacrosUpdate(ctx, statefulLacrosVersion.GetString(), statefulLacrosComponent, utsClient); err != nil {
		s.Fatal("Failed to verify provisioned Lacros version: ", err)
	}
}

// getRootfsLacrosVersion returns the version of Lacros Chrome in the Rootfs partition.
func getRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, utsClient proto.UpdateTestServiceClient) (version.Version, error) {
	req := &proto.GetBrowserVersionRequest{
		Browser: proto.BrowserType_LACROS_ROOTFS,
	}
	res, err := utsClient.GetBrowserVersion(ctx, req)
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}
	// Note that there must be only one Rootfs Lacros version on a DUT.
	return version.Parse(res.Versions[0]), nil
}

// provisionLacrosFromRootfsImage calls a RPC to the TLS to provision Stateful Lacros from Rootfs Lacros.
func provisionLacrosFromRootfsImage(ctx context.Context, tlsAddr string, dut *dut.DUT, overrideVersion, overrideComponent string) error {
	tlsClient, err := provision.Dial(ctx, tlsAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to TLS")
	}
	defer tlsClient.Close()

	dutName := dut.HostName()
	testing.ContextLogf(ctx, "Provisioning Lacros from Rootfs image: DUT=%v, overrideVersion=%v, overrideComponent=%v", dutName, overrideVersion, overrideComponent)
	return tlsClient.ProvisionLacrosFromDeviceFile(
		ctx, dutName, rootfsLacrosImageFileURL, overrideVersion, filepath.Join(statefulLacrosRootComponentPath, overrideComponent))
}

// verifyLacrosUpdate calls a RPC to the test service to verify the provisioned Lacros update is installed and selected in runtime on a DUT as expected.
func verifyLacrosUpdate(ctx context.Context, overrideVersion, overrideComponent string, utsClient lacros.UpdateTestServiceClient) error {
	// Build browser contexts for a test request.
	ashCtx := &proto.BrowserContext{
		Browser: proto.BrowserType_ASH,
		Opts: []string{
			"--enable-features=LacrosSupport",
			"--component-updater=url-source=" + bogusComponentUpdaterURL, // Block Component Updater.
		},
	}
	lacrosCtx := &proto.BrowserContext{
		Browser: proto.BrowserType_LACROS_STATEFUL,
	}

	// Send a test request to the DUT.
	res, err := utsClient.VerifyUpdate(ctx,
		&proto.VerifyUpdateRequest{
			AshContext:               ashCtx,
			ProvisionedLacrosContext: []*proto.BrowserContext{lacrosCtx},
			ExpectedBrowser:          proto.BrowserType_LACROS_STATEFUL,
			ExpectedVersion:          overrideVersion,
			ExpectedComponent:        overrideComponent,
			UseUi:                    true,
		})
	if err != nil {
		return errors.Wrap(err, "verifyLacrosUpdate: failed to verify version on Lacros")
	}
	if res.Result.Status != proto.TestResult_PASSED {
		return errors.Wrapf(err, "verifyLacrosUpdate: returns test failure status: %v", res.Result)
	}
	return nil
}

// clearLacrosUpdate calls a RPC to the test service to remove provisioned Lacros and reset to the previous state.
func clearLacrosUpdate(ctx context.Context, utsClient proto.UpdateTestServiceClient) error {
	if _, err := utsClient.ClearUpdate(ctx, &proto.ClearUpdateRequest{}); err != nil {
		return errors.Wrap(err, "clearLacrosUpdate: failed to clear provisioned Lacros")
	}
	return nil
}
