// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package update contains helper functions for Lacros update tests.
package update

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
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// GetAshVersion returns the version of Ash Chrome.
func GetAshVersion(ctx context.Context, dut *dut.DUT, utsClient lacrosservice.UpdateTestServiceClient) (version.Version, error) {
	res, err := utsClient.GetBrowserVersion(ctx,
		&lacrosservice.GetBrowserVersionRequest{
			Browser: lacrosservice.BrowserType_ASH,
		})
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getAshVersion")
	}

	if len(res.Versions) != 1 {
		return version.Version{}, errors.Wrapf(err, "expected only one Ash Chrome version. actual: %v", res.Versions)
	}
	v := version.Parse(res.Versions[0])
	if !v.IsValid() {
		return version.Version{}, errors.Wrap(err, "invalid Ash Chrome version")
	}
	return v, nil
}

// GetRootfsLacrosVersion returns the version of Lacros Chrome in the Rootfs partition.
func GetRootfsLacrosVersion(ctx context.Context, dut *dut.DUT, utsClient lacrosservice.UpdateTestServiceClient) (version.Version, error) {
	res, err := utsClient.GetBrowserVersion(ctx,
		&lacrosservice.GetBrowserVersionRequest{
			Browser: lacrosservice.BrowserType_LACROS_ROOTFS,
		})
	if err != nil {
		return version.Version{}, errors.Wrap(err, "failed to getRootfsLacrosVersion")
	}

	if len(res.Versions) != 1 {
		return version.Version{}, errors.Wrapf(err, "expected only one Rootfs Lacros version. actual: %v", res.Versions)
	}
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
func VerifyLacrosUpdate(ctx context.Context, expectedBrowser lacrosservice.BrowserType, expectedVersion, expectedComponent string, utsClient lacros.UpdateTestServiceClient) error {
	// Build browser contexts for a test request.
	ashCtx := &lacrosservice.BrowserContext{
		Browser: lacrosservice.BrowserType_ASH,
		Opts: []string{
			"--enable-features=LacrosSupport",
			"--component-updater=url-source=" + lacroscommon.BogusComponentUpdaterURL, // Block Component Updater.
			"--disable-lacros-keep-alive",                                             // Disable keep-alive for testing. See crbug.com/1268743.
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
			ExpectedBrowser:          expectedBrowser,
			ExpectedVersion:          expectedVersion,
			ExpectedComponent:        expectedComponent,
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

// SaveLogsFromDut saves device logs that are useful for troubleshooting test failures.
func SaveLogsFromDut(ctx context.Context, dut *dut.DUT, logOutDir string) {
	const logFileName = "lacros.log"

	logPathSrc := filepath.Join(lacroscommon.LacrosUserDataDir, logFileName)
	logPathDst := filepath.Join(logOutDir, logFileName)
	if err := linuxssh.GetFile(ctx, dut.Conn(), logPathSrc, logPathDst, linuxssh.PreserveSymlinks); err != nil {
		testing.ContextLogf(ctx, "Failed to save %s to %s. Error: %s", logPathSrc, logPathDst, err)
	}
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
		return lacroscommon.LacrosDevChannelName, nil
	}

	switch component {
	case
		lacroscommon.LacrosCanaryChannelName,
		lacroscommon.LacrosDevChannelName,
		lacroscommon.LacrosBetaChannelName,
		lacroscommon.LacrosStableChannelName:
		return component, nil
	default:
		return "", errors.New("Not supported component: " + component)
	}
}
