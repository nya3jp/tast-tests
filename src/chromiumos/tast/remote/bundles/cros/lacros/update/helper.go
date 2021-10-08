// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosupdate

import (
	"context"
	"path/filepath"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/lacros/provision"
	"chromiumos/tast/remote/bundles/cros/lacros/version"
	"chromiumos/tast/services/cros/lacros"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

// GetAshVersion returns the version of Ash Chrome.
func GetAshVersion(ctx context.Context, dut *dut.DUT, utsClient lacrosservice.UpdateTestServiceClient) (version.Version, error) {
	req := &lacrosservice.GetBrowserVersionRequest{
		Browser: lacrosservice.BrowserType_ASH,
	}
	res, err := utsClient.GetBrowserVersion(ctx, req)
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getAshVersion")
	}
	// Note that there must be only one Ash Chrome version on a DUT.
	v := version.Parse(res.Versions[0])
	if !v.IsValid() {
		return version.Version{}, errors.Wrap(err, "invalid Ash Chrome version")
	}
	return v, nil
}

// GetRootfsLacrosVersion returns the version of Lacros Chrome in the Rootfs partition.
func GetRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, utsClient lacrosservice.UpdateTestServiceClient) (version.Version, error) {
	req := &lacrosservice.GetBrowserVersionRequest{
		Browser: lacrosservice.BrowserType_LACROS_ROOTFS,
	}
	res, err := utsClient.GetBrowserVersion(ctx, req)
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}
	// Note that there must be only one Rootfs Lacros version on a DUT.
	v := version.Parse(res.Versions[0])
	if !v.IsValid() {
		return version.Version{}, errors.Wrap(err, "invalid Rootfs Lacros version")
	}
	return v, nil
}

// ProvisionLacrosFromRootfsLacrosImagePath calls a RPC to the TLS to provision Stateful Lacros from the source of Rootfs Lacros image installation path.
// This is useful when making the test hermetic by removing external source of Lacros binary.
func ProvisionLacrosFromRootfsLacrosImagePath(ctx context.Context, tlsAddr string, dut *dut.DUT, overrideVersion, overrideComponent string) error {
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

// VerifyLacrosUpdate calls a RPC to the test service to verify the provisioned Lacros update is installed and selected in runtime on a DUT as expected.
func VerifyLacrosUpdate(ctx context.Context, overrideVersion, overrideComponent string, utsClient lacros.UpdateTestServiceClient) error {
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

// ClearLacrosUpdate calls a RPC to the test service to remove provisioned Lacros and reset to the previous state.
func ClearLacrosUpdate(ctx context.Context, utsClient lacrosservice.UpdateTestServiceClient) error {
	if _, err := utsClient.ClearUpdate(ctx, &lacrosservice.ClearUpdateRequest{}); err != nil {
		return errors.Wrap(err, "clearLacrosUpdate: failed to clear provisioned Lacros")
	}
	return nil
}

// LacrosComponentVar returns the valid Lacros component to override from the runtime var. Defaults to Lacros dev channel if the var is not present.
func LacrosComponentVar(s *testing.State) (string, error) {
	component, ok := s.Var("lacrosComponent")
	if !ok {
		// Defaults to Lacros dev channel
		return "lacros-dogfood-dev", nil
	}
	switch component {
	case "lacros-dogfood-canary", "lacros-dogfood-dev", "lacros-dogfood-beta", "lacros-dogfood-stable":
		return component, nil
	default:
		return "", errors.New("Not supported component: " + component)
	}
}
