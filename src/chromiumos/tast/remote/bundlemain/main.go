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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/bundle"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
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
	r := reporters.New(s.DUT())

	rootPart, err := reporters.RootPartition(ctx, r)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get root partition")
	}

	removable, err := reporters.IsRemovableDevice(ctx, r, rootPart)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to determine if %q is removable", rootPart)
	}

	if removable {
		// Skipping this check when the device is boot from removable device.
		return nil
	}

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
	var err error
	var hwsecTpmStatus *hwsec.NonsensitiveStatusInfo
	hwsecDACounter := 0
	if s.DUT() != nil {
		// Store current DA value before running the tast.
		hwsecDACounter, err = hwsecGetDACounter(ctx, s)
		if err != nil {
			s.Log("Failed to get TPM DA counter: ", err)
			// Assume the counter value is zero when we failed to get the DA counter.
			hwsecDACounter = 0
		}
		// Store current TPM status before running the tast.
		hwsecTpmStatus, err = hwsecGetTPMStatus(ctx, s)
		if err != nil {
			s.Log("Failed to get TPM status: ", err)
			hwsecTpmStatus = nil
		}
	}

	return func(ctx context.Context, s *testing.TestHookState) {
		// Ensure that the DUT is connected.
		// Get output directory.
		dir, ok := testing.ContextOutDir(ctx)
		if !ok {
			s.Log("Failed to get name of output directory")
			return
		}
		dut := s.DUT()
		if dut != nil {
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

			// Get /var/log/messages from all DUTs.
			if err := downloadVarLogMessages(ctx, dir, dut); err != nil {
				s.Log("Download /var/log/messages failed from DUT (primary): ", err)
			}
		}
		for _, role := range s.CompanionDUTRoles() {
			cdut := s.CompanionDUT(role)
			if cdut != nil {
				continue
			}
			dirName := fmt.Sprintf("%v_%v", role, cdut.HostName())
			outputDir := filepath.Join(dir, dirName)
			if err := downloadVarLogMessages(ctx, outputDir, cdut); err != nil {
				s.Logf("Download /var/log/messages failed from DUT (%v): %v", role, err)
			}
		}

		// Only save faillog when there is an error.
		if !s.HasError() {
			return
		}

		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
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

		// Get name of target. Use a timestamp in the name to avoid
		// overwriting any existing files.
		timeStr := time.Now().Format("20060102-150405.000000")
		dst := filepath.Join(dir, "faillog", timeStr)

		// Create the parent directory if it doesn't already exist.
		if err := os.MkdirAll(dst, 0755); err != nil {
			s.Logf("Failed to create directory %v: %v", dst, err)
			return
		}

		// Transfer the file from DUT to host machine.
		if err := linuxssh.GetFile(ctx, dut.Conn(), res.Path, dst, linuxssh.PreserveSymlinks); err != nil {
			s.Logf("Failed to download %v from DUT to %v at local host: %v", res.Path, dst, err)
			return
		}
	}
}

// downloadVarLogMessages downloads /var/log/messages from a DUT.
func downloadVarLogMessages(ctx context.Context, outputDir string, dut *dut.DUT) error {
	if !dut.Connected(ctx) {
		if err := dut.WaitConnect(ctx); err != nil {
			return errors.Wrapf(err, "failed to connect to the DUT (%v)", dut.HostName())
		}
	}

	dst := filepath.Join(outputDir, "messages")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return errors.Errorf("failed to create directory %q to store /var/log/messages for DUT (%v)", outputDir, dut.HostName())
	}

	// Transfer messages file from DUT to host machine.
	if err := linuxssh.GetFile(ctx, dut.Conn(), "/var/log/messages", dst, linuxssh.PreserveSymlinks); err != nil {
		return errors.Wrapf(err, "failed to download /var/log/messages from DUT (%v) to %v at local host", dut.HostName(), dst)
	}

	return nil
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
		TestHook:     testHookRemote,
		BeforeReboot: beforeReboot,
	}))
}
