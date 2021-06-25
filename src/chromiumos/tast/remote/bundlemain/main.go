// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bundlemain provides a main function implementation for a bundle
// to share it from various remote bundle executables.
// The most of the frame implementation is in chromiumos/tast/bundle package,
// but some utilities, which lives in support libraries for maintenance,
// need to be injected.
package bundlemain

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/bundle"
	"chromiumos/tast/common/global"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func hwsecGetDACounter(ctx context.Context, s *testing.TestHookState) (int, error) {
	cmdRunner := hwsecremote.NewLoglessCmdRunner(s.DUT())
	tpmManager := hwsec.NewTPMManagerClient(cmdRunner)

	// Get the TPM dictionary attack info
	daInfo, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get the TPM dictionary attack info")
	}
	return daInfo.Counter, nil
}

func hwsecGetTPMStatus(ctx context.Context, s *testing.TestHookState) (*hwsec.NonsensitiveStatusInfo, error) {
	cmdRunner := hwsecremote.NewLoglessCmdRunner(s.DUT())
	tpmManager := hwsec.NewTPMManagerClient(cmdRunner)

	// Get the TPM nonsensitive status info
	status, err := tpmManager.GetNonsensitiveStatusIgnoreCache(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the TPM nonsensitive status info")
	}
	return status, nil
}

func hwsecCheckTPMState(ctx context.Context, s *testing.TestHookState, origStatus *hwsec.NonsensitiveStatusInfo, origCounter int) error {
	status, err := hwsecGetTPMStatus(ctx, s)
	if err != nil {
		return errors.Wrap(err, "failed to get TPM status")
	}

	// We didn't expect the TPM is owned but doesn't have the permission to reset DA counter.
	if status.IsOwned && !status.HasResetLockPermissions {
		testing.ContextLog(ctx, "TPM is owned but doesn't have the permission to reset DA counter")
		// But don't failed the tast if it's not cause by this tast.
		if origStatus == nil || !origStatus.IsOwned || origStatus.HasResetLockPermissions {
			return errors.Errorf("unexpect TPM status: %#v -> %#v", origStatus, status)
		}
	}

	// Only Check the DA counter when the TPM is owned.
	if status.IsOwned {
		da, err := hwsecGetDACounter(ctx, s)
		if err != nil {
			return errors.Wrap(err, "failed to get DA counter")
		}
		if da > origCounter {
			return errors.Errorf("TPM dictionary counter is increased: %v -> %v", origCounter, da)
		}
	}

	return nil
}

// testHookRemote returns a function that performs post-run activity after a test run is done.
func testHookRemote(ctx context.Context, s *testing.TestHookState) func(ctx context.Context,
	s *testing.TestHookState) {

	// Store current DA value before running the tast.
	hwsecDACounter, err := hwsecGetDACounter(ctx, s)
	if err != nil {
		s.Log("Failed to get TPM DA counter: ", err)
		// Assume the counter value is zero when we failed to get the DA counter.
		hwsecDACounter = 0
	}

	// Store current TPM status before running the tast.
	hwsecTpmStatus, err := hwsecGetTPMStatus(ctx, s)
	if err != nil {
		s.Log("Failed to get TPM status: ", err)
		hwsecTpmStatus = nil
	}

	return func(ctx context.Context, s *testing.TestHookState) {
		// Ensure that the DUT is connected.
		dut := s.DUT()
		if !dut.Connected(ctx) {
			if err := dut.WaitConnect(ctx); err != nil {
				s.Log("Failed to connect to the DUT: ", err)
				return
			}
		}

		// Ensure the TPM is in the expect state after tast finish.
		if err := hwsecCheckTPMState(ctx, s, hwsecTpmStatus, hwsecDACounter); err != nil {
			s.Error("Failed to check TPM state: ", err)
		}

		// Only save faillog when there is an error.
		if !s.HasError() {
			return
		}

		cl, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
		if err != nil {
			s.Log("Failed to connect to the RPC service on the DUT: ", err)
			return
		}
		defer cl.Close(ctx) // Close connection when everything is done.

		// Get the Faillog Service client.
		cr := baserpc.NewFaillogServiceClient(cl.Conn)

		// Ask Faillog service to create faillog and get the path as response.
		res, err := cr.Create(ctx, &empty.Empty{})
		if err != nil {
			s.Log("Failed to get faillog: ", err)
			return
		}

		// Ask Faillog Service to remove faillog directory at the DUT after it is downloaded.
		defer func() {
			if _, err := cr.Remove(ctx, &empty.Empty{}); err != nil {
				s.Log("Failed to remove faillog.tar.gz from DUT: ", err)
				return
			}
		}()
		if res.Path == "" {
			s.Log("Got empty path for faillog")
			return
		}

		// Get output directory.
		dir, ok := testing.ContextOutDir(ctx)
		if !ok {
			s.Log("Failed to get name of output directory")
			return
		}

		// Get name of target
		dst := filepath.Join(dir, "faillog")
		// Transfer the file from DUT to host machine.
		if err := linuxssh.GetFile(ctx, dut.Conn(), res.Path, dst, linuxssh.PreserveSymlinks); err != nil {
			s.Logf("Failed to download %v from DUT to %v at local host: %v", res.Path, dst, err)
			return
		}
	}
}

func beforeReboot(ctx context.Context, d *dut.DUT) error {
	// Copy logs before reboot. Ignore errors on failure.
	testOutDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		// TODO(crbug.com/1097657): Return error after making sure existing tests does not get flaky by this check.
		return nil
	}
	dateString := time.Now().Format(time.RFC3339)
	outDir := filepath.Join(testOutDir, "reboots", dateString)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		testing.ContextLog(ctx, "Failed to make output subdirectory: ", err)
	}
	if err := d.GetFile(ctx, "/var/log/messages", filepath.Join(outDir, "messages")); err != nil {
		testing.ContextLog(ctx, "Failed to copy syslog: ", err)
	}
	return nil
}

// RunRemote is an entry point function for remote bundles.
func RunRemote() {
	os.Exit(bundle.RemoteDefault(bundle.Delegate{
		TestHook:             testHookRemote,
		BeforeReboot:         beforeReboot,
		InitializeGlobalVars: global.InitializeGlobalVars,
	}))
}
