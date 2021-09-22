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
	"chromiumos/tast/remote/bundles/cros/lacros/autoupdateutil"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

const (
	statefulLacrosRootComponentPath = "/home/chronos/cros-components/"
	statefulLacrosComponent         = "lacros-dogfood-dev"
	rootfsLacrosImageFileURL        = "file:///opt/google/lacros"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoupdateRootfsToStateful,
		Desc:         "Tests that Stateful Lacros is selected when it is newer than Rootfs Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.AutoupdateTestService"},
		Timeout:      10 * time.Minute,
		// Vars:         []string{"provisionServerAddr"},
	})
}

func AutoupdateRootfsToStateful(ctx context.Context, s *testing.State) {
	// Set up a RPC client to the update test service on a DUT.
	dut := s.DUT()
	conn, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	autsClient := lacros.NewAutoupdateTestServiceClient(conn.Conn)

	// Bump up the major version of Stateful Lacros to be newer than of Rootfs
	// one in order to simulate the desired test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := getRootfsLacrosVersion(ctx, dut, autsClient)
	if err != nil || !rootfsLacrosVersion.IsValid() {
		s.Fatal("Failed to get the Rootfs Lacros version: ", err)
	}
	statefulLacrosVersion := rootfsLacrosVersion
	// TODO: Stay within supported version skew range of [0, +2] milestone from Ash, not +9000.
	statefulLacrosVersion.Increment(9000, 0, 0, 0)

	// Reserve time for clean-up
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	// Provision Stateful Lacros from the device image file with the simulated
	// version and component.
	if err := provisionLacrosFromRootfsImage(ctx, autoupdateutil.TLSAddrVar.Value(), dut, statefulLacrosVersion.GetString(), statefulLacrosComponent); err != nil {
		// Note that s.Error is used instead of s.Fatal to proceed for clean up before exit.
		s.Error("Failed to provision Stateful Lacros from Rootfs image source: ", err)
	}
	defer func(ctx context.Context) {
		if err := cleanupLacros(ctx, dut); err != nil {
			s.Fatal("Failed to clean up provisioned Lacros: ", err)
		}
	}(ctxForCleanup)

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := verifyLacrosVersion(ctx, dut, statefulLacrosVersion.GetString(), statefulLacrosComponent, autsClient); err != nil {
		s.Error("Failed to verify provisioned Lacros version: ", err)
	}
}

func getRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, autsClient lacros.AutoupdateTestServiceClient) (version.Version, error) {
	req := &lacros.GetBrowserVersionRequest{
		Browser: lacros.BrowserType_LACROS_ROOTFS,
	}
	res, err := autsClient.GetBrowserVersion(ctx, req)
	if err != nil || len(res.Versions) == 0 {
		return version.Version{}, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}
	return version.Parse(res.Versions[0]), nil
}

func provisionLacrosFromRootfsImage(ctx context.Context, tlsAddr string, dut *dut.DUT, overrideVersion, overrideComponent string) error {
	tlsClient, err := autoupdateutil.Dial(ctx, tlsAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to TLS")
	}
	defer tlsClient.Close()

	dutName := dut.HostName()
	testing.ContextLogf(ctx, "Provisioning Lacros from Rootfs image: DUT=%v, overrideVersion=%v, overrideComponent=%v", dutName, overrideVersion, overrideComponent)
	return tlsClient.ProvisionLacrosFromDeviceFile(
		ctx, dutName, rootfsLacrosImageFileURL, overrideVersion, filepath.Join(statefulLacrosRootComponentPath, overrideComponent))
}

func cleanupLacros(ctx context.Context, dut *dut.DUT) error {
	dutConn := dut.Conn()
	if err := dutConn.CommandContext(ctx, "rm", "-rf", statefulLacrosRootComponentPath).Run(); err != nil {
		return errors.Wrap(err, "cleanupLacros: failed to remove CrOS components")
	}
	// Mark that the stateful partition is corrupted, so the provision server can restore it.
	if err := dutConn.CommandContext(ctx, "touch", "/mnt/stateful_partition/.corrupt_stateful").Run(); err != nil {
		return errors.Wrap(err, "cleanupLacros: failed to mark that the stateful is corrupted")
	}
	return nil
}

func verifyLacrosVersion(ctx context.Context, dut *dut.DUT, overrideVersion, overrideComponent string, autsClient lacros.AutoupdateTestServiceClient) error {
	// Build browser contexts for a test request.
	ashCtx := &lacros.BrowserContext{
		Browser: lacros.BrowserType_ASH,
		Opts: []string{
			"--enable-features=LacrosSupport",
			"--component-updater=url-source=http://localhost:12345", // Block Component Updater.
		},
	}
	lacrosCtx := &lacros.BrowserContext{
		Browser: lacros.BrowserType_LACROS_STATEFUL,
	}

	// Send a test request to the DUT.
	res, err := autsClient.VerifyLacrosVersion(ctx,
		&lacros.VerifyLacrosVersionRequest{
			AshContext:               ashCtx,
			ProvisionedLacrosContext: []*lacros.BrowserContext{lacrosCtx},
			ExpectedBrowser:          lacros.BrowserType_LACROS_STATEFUL,
			ExpectedVersion:          overrideVersion,   // "9999.0.0.0"
			ExpectedComponent:        overrideComponent, // "lacros-dogfood-dev"
		})
	if err != nil {
		return errors.Wrap(err, "verifyLacrosVersion: failed to verify version on Lacros")
	}
	if res.Result.Status != lacros.TestResult_PASSED {
		return errors.Wrapf(err, "verifyLacrosVersion: returns test failure status: %v", res.Result)
	}
	return nil
}
