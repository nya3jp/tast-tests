// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	tmpb "chromiumos/system_api/tpm_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// CmdRunner declares interface that runs command on DUT.
type CmdRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// Helper provides various helper functions that could be shared across all
// hwsec integration test regardless of run-type, i.e., remote or local.
type Helper struct {
	cmdRunner        CmdRunner
	tpmClearer       TPMClearer
	cryptohomeUtil   *UtilityCryptohomeBinary
	tpmManagerUtil   *UtilityTpmManagerBinary
	daemonController *DaemonController
}

// NewHelper creates a new Helper, with r responsible for CmdRunner.
func NewHelper(r CmdRunner, t TPMClearer) (*Helper, error) {
	cryptohomeUtil, err := NewUtilityCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	tpmManagerUtil, err := NewUtilityTpmManagerBinary(r)
	if err != nil {
		return nil, err
	}
	daemonController := NewDaemonController(r)
	return &Helper{
		cmdRunner:        r,
		tpmClearer:       t,
		cryptohomeUtil:   cryptohomeUtil,
		tpmManagerUtil:   tpmManagerUtil,
		daemonController: daemonController,
	}, nil
}

// CmdRunner exposes the cmdRunner of helper
func (h *Helper) CmdRunner() CmdRunner { return h.cmdRunner }

// TPMClearer exposes the tpmClearer of helper
func (h *Helper) TPMClearer() TPMClearer { return h.tpmClearer }

// CryptohomeUtil exposes the cryptohomeUtil of helper
func (h *Helper) CryptohomeUtil() *UtilityCryptohomeBinary { return h.cryptohomeUtil }

// TPMManagerUtil exposes the tpmManagerUtil of helper
func (h *Helper) TPMManagerUtil() *UtilityTpmManagerBinary { return h.tpmManagerUtil }

// DaemonController exposes the daemonController of helper
func (h *Helper) DaemonController() *DaemonController { return h.daemonController }

// EnsureTPMIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error.
func (h *Helper) EnsureTPMIsReady(ctx context.Context, timeout time.Duration) error {
	info, err := h.tpmManagerUtil.GetNonsensitiveStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |GetNonsensitiveStatus|")
	}
	if !info.IsOwned {
		if _, err := h.tpmManagerUtil.TakeOwnership(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
	}
	return testing.Poll(ctx, func(context.Context) error {
		info, err := h.tpmManagerUtil.GetNonsensitiveStatus(ctx)
		if err != nil {
			return errors.New("error during checking TPM readiness")
		}
		if info.IsOwned {
			return nil
		}
		return errors.New("haven't confirmed to be owned")
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: PollingInterval,
	})
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment
// when the function returns |nil|. Otherwise, returns any encountered error.
func (h *Helper) EnsureIsPreparedForEnrollment(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(context.Context) error {
		// intentionally ignores error; retry the operation until timeout.
		isPrepared, err := h.cryptohomeUtil.IsPreparedForEnrollment(ctx)
		if err != nil {
			return err
		}
		if !isPrepared {
			return errors.New("not prepared yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: PollingInterval,
	})
}

// RemoveFile would delete the file
func (h *Helper) RemoveFile(ctx context.Context, filename string) error {
	_, err := h.cmdRunner.Run(ctx, "rm", "-f", "--", filename)
	return err
}

// ReadFile would read data from the file
func (h *Helper) ReadFile(ctx context.Context, filename string) ([]byte, error) {
	return h.cmdRunner.Run(ctx, "cat", "--", filename)
}

// WriteFile would write data into the file
func (h *Helper) WriteFile(ctx context.Context, filename string, data []byte) error {
	// Because we may pass NULL('\0') byte in data, using base64 to encode the data could resolve the escape character issue.
	// Using "echo" or "printf" would require a complex escaping rule to let it work correctly.
	b64String := base64.StdEncoding.EncodeToString(data)
	echoStrCmd := fmt.Sprintf("echo %s", shutil.Escape(b64String))
	b64DecCmd := fmt.Sprintf("base64 -d > %s", shutil.Escape(filename))
	cmd := fmt.Sprintf("%s | %s", echoStrCmd, b64DecCmd)
	if _, err := h.cmdRunner.Run(ctx, "sh", "-c", cmd); err != nil {
		return errors.Wrap(err, "failed to echo string")
	}
	return nil
}

// GetTPMManagerLocalData would read the tpm_manager local_tpm_data.
// Note: Get the data without stopping tpm_managerd may result stale data.
func (h *Helper) GetTPMManagerLocalData(ctx context.Context) ([]byte, error) {
	return h.ReadFile(ctx, "/var/lib/tpm_manager/local_tpm_data")
}

// SetTPMManagerLocalData would write the local_tpm_data.
// Because tpm_managerd may cache the local data in the memory, we would need to restart tpm_managerd after modifying the data.
func (h *Helper) SetTPMManagerLocalData(ctx context.Context, data []byte) error {
	return h.WriteFile(ctx, "/var/lib/tpm_manager/local_tpm_data", data)
}

// DropResetLockPermissions drops the reset lock permissions and return a callback to restore the permissions.
func (h *Helper) DropResetLockPermissions(ctx context.Context) (restoreFunc func(ctx context.Context) error, retErr error) {
	// Stop TPM Manager before modifying its local data.
	if err := h.daemonController.Stop(ctx, TPMManagerDaemonInfo); err != nil {
		return nil, errors.Wrap(err, "failed to stop TPM Manager")
	}

	// Restart it after finishing all operation.
	defer func() {
		if err := h.daemonController.Start(ctx, TPMManagerDaemonInfo); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to start TPM Manager")
			} else {
				testing.ContextLog(ctx, "Failed to take screenshot: ", err)
			}
		}
	}()

	rawData, err := h.GetTPMManagerLocalData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get local TPM data")
	}

	var data tmpb.LocalData
	if err := proto.Unmarshal(rawData, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal local TPM data")
	}

	// Drop the owner password, so tpm_manager couldn't use it to create owner delegate on TPM1.2 device.
	data.OwnerPassword = []byte{}
	// Drop the owner delegate, so tpm_manager couldn't use it to reset DA counter on TPM1.2 device.
	data.OwnerDelegate = &tmpb.AuthDelegate{}

	// Drop the lockout password, so tpm_manager couldn't use it to reset DA counter on TPM2.0 device.
	data.LockoutPassword = []byte{}

	newData, err := proto.Marshal(&data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal local TPM data")
	}

	// Write back the data into the local data of tpm_manager.
	if err := h.SetTPMManagerLocalData(ctx, newData); err != nil {
		return nil, errors.Wrap(err, "failed to set local TPM data")
	}

	return func(ctx context.Context) error {
		// Stop TPM Manager before modifying its local data.
		if err := h.daemonController.Stop(ctx, TPMManagerDaemonInfo); err != nil {
			return errors.Wrap(err, "failed to stop TPM Manager")
		}

		// Restart it after finishing all operation.
		defer func() {
			if err := h.daemonController.Start(ctx, TPMManagerDaemonInfo); err != nil {
				if retErr == nil {
					retErr = errors.Wrap(err, "failed to start TPM Manager")
				} else {
					testing.ContextLog(ctx, "Failed to take screenshot: ", err)
				}
			}
		}()

		// Restore the local data.
		if err := h.SetTPMManagerLocalData(ctx, rawData); err != nil {
			return errors.Wrap(err, "failed to restore local TPM data")
		}
		return nil
	}, nil
}

// GetTPMVersion would rteurn the TPM version, for example: "1.2", "2.0"
func (h *Helper) GetTPMVersion(ctx context.Context) (string, error) {
	out, err := h.cmdRunner.Run(ctx, "tpmc", "tpmver")
	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(out)), err
}

// ensureTPMIsReset ensures the TPM is reset when the function returns nil.
// Otherwise, returns any encountered error.
// Optionally removes files from the DUT to simulate a powerwash.
func (h *Helper) ensureTPMIsReset(ctx context.Context, removeFiles bool) error {
	if err := h.daemonController.WaitForAllDBusServices(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for hwsec D-Bus services to be ready")
	}

	if err := h.daemonController.StopAllServices(ctx); err != nil{
		return errors.Wrap(err, "")
	}

	if removeFiles {
		if out, err := h.cmdRunner.Run(ctx, "rm", "-rf", "--",
			"/home/.shadow/",
			"/home/chronos/.oobe_completed",
			"/home/chronos/Local State",
			"/mnt/stateful_partition/.tpm_owned",
			"/mnt/stateful_partition/.tpm_status.sum",
			"/mnt/stateful_partition/.tpm_status",
			"/run/cryptohome",
			"/run/lockbox",
			"/run/tpm_manager",
			"/var/cache/app_pack",
			"/var/cache/shill/default.profile",
			"/var/lib/boot-lockbox",
			"/var/lib/bootlockbox",
			"/var/lib/chaps",
			"/var/lib/cryptohome",
			"/var/lib/public_mount_salt",
			"/var/lib/tpm_manager",
			"/var/lib/tpm",
			"/var/lib/u2f",
			"/var/lib/whitelist/",
		); err != nil {
			// TODO(b/173189029): Ignore errors on failure. This is a workaround to prevent Permission denied when removing a fscrypt directory.
			testing.ContextLog(ctx, "Failed to remove files to clear ownership: ", err, string(out))
		}
	}

	if err := h.tpmClearer.ClearTPM(ctx); err != nil{
		return errors.Wrap(err, "")
	}

	if err := h.daemonController.EnsureAllServices(ctx); err != nil{
		return errors.Wrap(err, "")
	}

	testing.ContextLog(ctx, "Waiting for system to be ready after reset TPM ")
	info, err := h.TPMManagerUtil().GetNonsensitiveStatus(ctx)
	if err != nil {
		return errors.Wrap(err,"failed to get TPM status")
	}
	if info.IsOwned {
		// If the TPM is ready, the reset was not successful
		return errors.New("ineffective reset of TPM")
	}

	return nil
}

// EnsureTPMIsReset ensures the TPM is reset when the function returns nil.
// Otherwise, returns any encountered error.
func (h *Helper) EnsureTPMIsReset(ctx context.Context) error {
	return h.ensureTPMIsReset(ctx, false)
}

// EnsureTPMIsResetAndPowerwash ensures the TPM is reset and simulates a Powerwash.
func (h *Helper) EnsureTPMIsResetAndPowerwash(ctx context.Context) error {
	return h.ensureTPMIsReset(ctx, true)
}
