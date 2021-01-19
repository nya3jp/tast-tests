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
	cmdRunner, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		return 0, errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Get the TPM dictionary attack info
	daInfo, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get the TPM dictionary attack info")
	}
	return daInfo.Counter, nil
}

func hwsecGetTPMStatus(ctx context.Context, s *testing.TestHookState) (*hwsec.NonsensitiveStatusInfo, error) {
	cmdRunner, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{}, errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{}, errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Get the TPM nonsensitive status info
	status, err := tpmManagerUtil.GetNonsensitiveStatus(ctx)
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{}, errors.Wrap(err, "failed to get the TPM nonsensitive status info")
	}
	return status, nil
}

func hwsecCheckDACounter(ctx context.Context, s *testing.TestHookState, origVal int) error {
	da, err := hwsecGetDACounter(ctx, s)
	if err != nil {
		return errors.Wrap(err, "failed to get DA counter")
	}
	if da > origVal {
		return errors.Errorf("TPM dictionary counter is increased: %v -> %v", origVal, da)
	}
	return nil
}

func hwsecCheckTPMStatus(ctx context.Context, s *testing.TestHookState, origStatus *hwsec.NonsensitiveStatusInfo) error {
	status, err := hwsecGetTPMStatus(ctx, s)
	if err != nil {
		return errors.Wrap(err, "failed to get TPM status")
	}
	if status.IsOwned && !status.HasResetLockPermissions {
		if !origStatus.IsOwned || origStatus.HasResetLockPermissions {
			return errors.Errorf("Unexpect TPM status: %#v -> %#v", origStatus, status)
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
	}

	// Store current TPM status before running the tast.
	hwsecTpmStatus, err := hwsecGetTPMStatus(ctx, s)
	if err != nil {
		s.Log("Failed to get TPM status: ", err)
	}

	return func(ctx context.Context, s *testing.TestHookState) {

		// Ensure the TPM is in the expect status after tast finish.
		if err := hwsecCheckTPMStatus(ctx, s, hwsecTpmStatus); err != nil {
			s.Error("Failed to check TPM status: ", err)
		}

		// Ensure the TPM dictionary attack counter didn't be increased after tast finish.
		if err := hwsecCheckDACounter(ctx, s, hwsecDACounter); err != nil {
			s.Error("Failed to check TPM DA counter: ", err)
		}

		// Only save faillog when there is an error.
		if !s.HasError() {
			return
		}

		// Connect to the DUT.
		dut := s.DUT()
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
		if err := linuxssh.GetFile(ctx, dut.Conn(), res.Path, dst); err != nil {
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
	os.Exit(bundle.RemoteDefault(bundle.RemoteDelegate{
		TestHook:     testHookRemote,
		BeforeReboot: beforeReboot,
	}))
}
