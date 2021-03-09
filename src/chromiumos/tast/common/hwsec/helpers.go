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

// CmdHelper provides various helper functions that could be shared across all
// hwsec integration test base on CmdRunner.
type CmdHelper struct {
	cmdRunner        CmdRunner
	cryptohome       *CryptohomeClient
	tpmManager       *TPMManagerClient
	daemonController *DaemonController
}

// AttestationHelper provides various helper functions that could be shared across all
// hwsec integration test base on AttestationClient.
type AttestationHelper struct {
	attestation *AttestationClient
}

// FullHelper is the full version of all kinds of helper that could be shared across all
// hwsec integration test regardless of run-type, i.e., remote or local.
type FullHelper struct {
	CmdHelper
	AttestationHelper
}

// NewCmdHelper creates a new CmdHelper, with r responsible for CmdRunner.
func NewCmdHelper(r CmdRunner) *CmdHelper {
	return &CmdHelper{
		cmdRunner:        r,
		cryptohome:       NewCryptohomeClient(r),
		tpmManager:       NewTPMManagerClient(r),
		daemonController: NewDaemonController(r),
	}
}

// NewAttestationHelper creates a new AttestationHelper, with ac responsible for AttestationDBus.
func NewAttestationHelper(ac AttestationDBus) *AttestationHelper {
	return &AttestationHelper{
		attestation: NewAttestationClient(ac),
	}
}

// NewFullHelper creates a new FullHelper, with ch responsible for CmdHelper and ah responsible for AttestationHelper.
func NewFullHelper(ch *CmdHelper, ah *AttestationHelper) *FullHelper {
	return &FullHelper{*ch, *ah}
}

// CmdRunner exposes the cmdRunner of helper
func (h *CmdHelper) CmdRunner() CmdRunner { return h.cmdRunner }

// CryptohomeClient exposes the cryptohome of helper
func (h *CmdHelper) CryptohomeClient() *CryptohomeClient { return h.cryptohome }

// TPMManagerClient exposes the tpmManager of helper
func (h *CmdHelper) TPMManagerClient() *TPMManagerClient { return h.tpmManager }

// DaemonController exposes the daemonController of helper
func (h *CmdHelper) DaemonController() *DaemonController { return h.daemonController }

// AttestationClient exposes the attestation of helper
func (h *AttestationHelper) AttestationClient() *AttestationClient { return h.attestation }

// EnsureTPMIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error.
func (h *CmdHelper) EnsureTPMIsReady(ctx context.Context, timeout time.Duration) error {
	info, err := h.tpmManager.GetNonsensitiveStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |GetNonsensitiveStatus|")
	}
	if !info.IsOwned {
		if _, err := h.tpmManager.TakeOwnership(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
	}
	return testing.Poll(ctx, func(context.Context) error {
		info, err := h.tpmManager.GetNonsensitiveStatus(ctx)
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
func (h *AttestationHelper) EnsureIsPreparedForEnrollment(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(context.Context) error {
		// intentionally ignores error; retry the operation until timeout.
		isPrepared, err := h.attestation.IsPreparedForEnrollment(ctx)
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
func (h *CmdHelper) RemoveFile(ctx context.Context, filename string) error {
	_, err := h.cmdRunner.Run(ctx, "rm", "-f", "--", filename)
	return err
}

// ReadFile would read data from the file
func (h *CmdHelper) ReadFile(ctx context.Context, filename string) ([]byte, error) {
	return h.cmdRunner.Run(ctx, "cat", "--", filename)
}

// WriteFile would write data into the file
func (h *CmdHelper) WriteFile(ctx context.Context, filename string, data []byte) error {
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
func (h *CmdHelper) GetTPMManagerLocalData(ctx context.Context) ([]byte, error) {
	return h.ReadFile(ctx, "/var/lib/tpm_manager/local_tpm_data")
}

// SetTPMManagerLocalData would write the local_tpm_data.
// Because tpm_managerd may cache the local data in the memory, we would need to restart tpm_managerd after modifying the data.
func (h *CmdHelper) SetTPMManagerLocalData(ctx context.Context, data []byte) error {
	return h.WriteFile(ctx, "/var/lib/tpm_manager/local_tpm_data", data)
}

// DropResetLockPermissions drops the reset lock permissions and return a callback to restore the permissions.
func (h *CmdHelper) DropResetLockPermissions(ctx context.Context) (restoreFunc func(ctx context.Context) error, retErr error) {
	// Stop TPM Manager before modifying its local data.
	if err := h.daemonController.Stop(ctx, TPMManagerDaemon); err != nil {
		return nil, errors.Wrap(err, "failed to stop TPM Manager")
	}

	// Restart it after finishing all operation.
	defer func() {
		if err := h.daemonController.Start(ctx, TPMManagerDaemon); err != nil {
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
		if err := h.daemonController.Stop(ctx, TPMManagerDaemon); err != nil {
			return errors.Wrap(err, "failed to stop TPM Manager")
		}

		// Restart it after finishing all operation.
		defer func() {
			if err := h.daemonController.Start(ctx, TPMManagerDaemon); err != nil {
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
func (h *CmdHelper) GetTPMVersion(ctx context.Context) (string, error) {
	out, err := h.cmdRunner.Run(ctx, "tpmc", "tpmver")
	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(out)), err
}
