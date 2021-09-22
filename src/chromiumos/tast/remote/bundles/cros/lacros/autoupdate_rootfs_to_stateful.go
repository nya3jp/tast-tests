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

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoupdateRootfsToStateful,
		Desc:         "Tests that Stateful Lacros is selected when it is newer than Rootfs Lacros",
		Contacts:     []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		ServiceDeps:  []string{"tast.cros.lacros.AutoupdateTestService"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"provisionServerAddr"},
	})
}

func AutoupdateRootfsToStateful(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	s.Log("DUT=", dut)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		// Add any clean up.
		unprovisionLacros(ctx, dut)
	}(cleanupCtx)

	// Set up a RPC client to the autoupdate test service on a DUT.
	conn, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Error("Failed to connect to DUT: ", err)
	}
	defer conn.Close(ctx)
	autsClient := lacros.NewAutoupdateTestServiceClient(conn.Conn)

	// Bump up the major version of Stateful Lacros to be newer than of Rootfs
	// Lacros in order to simulate the test scenario (Rootfs => Stateful).
	rootfsLacrosVersion, err := getRootfsLacrosVersion(ctx, dut, autsClient)
	if err != nil {
		s.Error("Failed to get the Rootfs Lacros version: ", err)
	}
	statefulLacrosVersion := rootfsLacrosVersion.Increment(9000, 0, 0, 0)

	// Provision Stateful Lacros from the device image file with the simulated
	// version and component.
	const statefulLacrosComponent = "lacros-dogfood-dev"
	tlsAddr, ok := s.Var("provisionServerAddr")
	if !ok {
		tlsAddr = autoupdateutil.TLSAddrDefault
	}
	if err := provisionLacrosFromRootfsLacros(ctx, tlsAddr, dut, statefulLacrosVersion.GetString(), statefulLacrosComponent); err != nil {
		s.Error("Failed to provision Stateful Lacros from Rootfs: ", err)
	}

	// Verify that the expected Stateful Lacros version/component is selected.
	if err := verifyLacrosVersion(ctx, dut, statefulLacrosVersion.GetString(), statefulLacrosComponent, autsClient); err != nil {
		s.Error("Failed to verify Lacros version: ", err)
	}
}

func getRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, autsClient lacros.AutoupdateTestServiceClient) (*version.Version, error) {
	req := &lacros.GetBrowserVersionRequest{Browser: lacros.BrowserType_LACROS_ROOTFS}
	res, err := autsClient.GetBrowserVersion(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}
	return version.Parse(res.Versions[0]), nil
}

func provisionLacrosFromRootfsLacros(ctx context.Context, tlsAddr string, dut *dut.DUT, overrideVersion, overrideComponent string) error {
	dutName := dut.HostName()
	// TODO: Remove log.
	testing.ContextLogf(ctx, "provisionLacros: DUT=%v, overrideVersion=%v, overrideComponent=%v", dutName, overrideVersion, overrideComponent)

	tlsClient, err := autoupdateutil.Dial(ctx, tlsAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to TLS")
	}
	defer tlsClient.Close()
	const rootfsLacrosImagePath = "file:///opt/google/lacros"
	return tlsClient.ProvisionLacrosFromDeviceFile(
		ctx, dutName, rootfsLacrosImagePath, overrideVersion, filepath.Join("/home/chronos/cros-components/", overrideComponent))
}

func unprovisionLacros(ctx context.Context, dut *dut.DUT) error {
	dutConn := dut.Conn()
	if err := dutConn.CommandContext(ctx, "rm", "-rf", "/home/chronos/cros-components/").Run(); err != nil {
		return errors.New("unprovisionLacros: failed to remove CrOS components")
	}
	// Mark that the stateful partition is corrupted, so the provision server can restore it.
	if err := dutConn.CommandContext(ctx, "touch", "/mnt/stateful_partition/.corrupt_stateful/").Run(); err != nil {
		return errors.New("unprovisionLacros: failed to mark that the stateful is corrupted")
	}
	// TODO: Remove log.
	testing.ContextLog(ctx, "unprovisionLacros ended")
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
	// TODO: Remove log.
	testing.ContextLogf(ctx, "verifyLacrosVersion: res=%v", res.Result)
	return nil
}
