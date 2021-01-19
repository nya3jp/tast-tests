// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"

	tmpb "chromiumos/system_api/tpm_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// NVRAMAttributeWriteAuth is used by DefineSpace to indicate that writing this NVRAM index requires authorization with authValue.
	NVRAMAttributeWriteAuth = "WRITE_AUTHORIZATION"

	// NVRAMAttributeReadAuth is used by DefineSpace to indicate that reading this NVRAM index requires authorization with authValue.
	NVRAMAttributeReadAuth = "READ_AUTHORIZATION"

	// tpmManagerNVRAMSuccessMessage is the error code from NVRAM related tpm_manager API when the operation is successful.
	tpmManagerNVRAMSuccessMessage = "NVRAM_RESULT_SUCCESS"

	// tpmManagerStatusSuccessMessage is the error code from other tpm_manager API when the operation is successful.
	tpmManagerStatusSuccessMessage = "STATUS_SUCCESS"
)

// UtilityTpmManagerBinary wraps and the functions of TpmManagerBinary and parses the outputs to
// structured data.
type UtilityTpmManagerBinary struct {
	binary *TpmManagerBinary
}

// NewUtilityTpmManagerBinary creates a new UtilityTpmManagerBinary.
func NewUtilityTpmManagerBinary(r CmdRunner) (*UtilityTpmManagerBinary, error) {
	binary, err := NewTpmManagerBinary(r)
	if err != nil {
		return nil, err
	}
	return &UtilityTpmManagerBinary{binary}, nil
}

// checkCommandAndReturn is a simple helper that checks if binaryMsg returned is successful, and returns the corresponding message and error.
func checkCommandAndReturn(ctx context.Context, binaryMsg []byte, err error, command, successMsg string) (string, error) {
	// Convert msg first because it's still used when there's an error.
	msg := string(binaryMsg)

	// Check if the command succeeds.
	if err != nil {
		// Command failed.
		return msg, errors.Wrapf(err, "calling %s failed with message %q", command, msg)
	}

	// Check if the message make sense.
	if !strings.Contains(msg, successMsg) {
		return msg, errors.Errorf("calling %s failed due to missing wanted context %q in stdout %q", command, successMsg, msg)
	}

	return msg, nil
}

// checkNVRAMCommandAndReturn is a simple helper that checks if binaryMsg returned from NVRAM related command is successful, and returns the corresponding message and error.
func checkNVRAMCommandAndReturn(ctx context.Context, binaryMsg []byte, err error, command string) (string, error) {
	return checkCommandAndReturn(ctx, binaryMsg, err, command, tpmManagerNVRAMSuccessMessage)
}

// checkStatusCommandAndReturn is a simple helper that checks if binaryMsg returned from status related command is successful, and returns the corresponding message and error.
func checkStatusCommandAndReturn(ctx context.Context, binaryMsg []byte, err error, command string) (string, error) {
	return checkCommandAndReturn(ctx, binaryMsg, err, command, tpmManagerStatusSuccessMessage)
}

// DefineSpace defines (creates) an NVRAM space at index, of size size, with attributes attributes and password password, and the NVRAM space will be bound to PCR0 if bindToPCR0 is true.
// If password is "", it'll not be passed to the command. attributes should be a slice that contains only the const NVRAMAttribute*.
// Will return nil for error iff the operation completes successfully. The string returned, msg, is the message from the command line, if any.
func (u *UtilityTpmManagerBinary) DefineSpace(ctx context.Context, size int, bindToPCR0 bool, index string, attributes []string, password string) (string, error) {
	attributesParam := strings.Join(attributes, "|")
	binaryMsg, err := u.binary.DefineSpace(ctx, size, bindToPCR0, index, attributesParam, password)

	return checkNVRAMCommandAndReturn(ctx, binaryMsg, err, "DefineSpace")
}

// DestroySpace destroys (removes) an NVRAM space at index.
// Will return nil for error iff the operation completes successfully. The string returned, msg, is the message from the command line, if any.
func (u *UtilityTpmManagerBinary) DestroySpace(ctx context.Context, index string) (string, error) {
	binaryMsg, err := u.binary.DestroySpace(ctx, index)

	return checkNVRAMCommandAndReturn(ctx, binaryMsg, err, "DestroySpace")
}

// WriteSpaceFromFile writes the content of file inputFile into the NVRAM space at index, with password password (if not empty).
func (u *UtilityTpmManagerBinary) WriteSpaceFromFile(ctx context.Context, index, inputFile, password string) (string, error) {
	binaryMsg, err := u.binary.WriteSpace(ctx, index, inputFile, password)

	return checkNVRAMCommandAndReturn(ctx, binaryMsg, err, "WriteSpace")
}

// ReadSpaceToFile reads the content of NVRAM space at index into the file outputFile, with password (if not empty).
func (u *UtilityTpmManagerBinary) ReadSpaceToFile(ctx context.Context, index, outputFile, password string) (string, error) {
	binaryMsg, err := u.binary.ReadSpace(ctx, index, outputFile, password)

	return checkNVRAMCommandAndReturn(ctx, binaryMsg, err, "ReadSpace")
}

// TakeOwnership takes the TPM ownership.
func (u *UtilityTpmManagerBinary) TakeOwnership(ctx context.Context) (string, error) {
	binaryMsg, err := u.binary.TakeOwnership(ctx)

	return checkStatusCommandAndReturn(ctx, binaryMsg, err, "TakeOwnership")
}

// Status returns the status string.
func (u *UtilityTpmManagerBinary) Status(ctx context.Context) (string, error) {
	binaryMsg, err := u.binary.Status(ctx)

	return checkStatusCommandAndReturn(ctx, binaryMsg, err, "Status")
}

// NonsensitiveStatusInfo contains the dictionary attack related information.
type NonsensitiveStatusInfo struct {
	// Whether a TPM is enabled on the system.
	IsEnabled bool

	// Whether the TPM has been owned.
	IsOwned bool

	// Whether the owner password is still retained.
	IsOwnerPasswordPresent bool

	// Whether tpm manager is capable of reset DA.
	HasResetLockPermissions bool
}

func parseStringMap(ctx context.Context, msg string, checkMatch bool, prefixes []string) (map[string]string, error) {
	lines := strings.Split(msg, "\n")
	parsed := map[string]string{}
	// TODO(yich): This compare is slow when we have big msg and prefixes.
	for _, line := range lines {
		for _, prefix := range prefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			if _, found := parsed[prefix]; found {
				testing.ContextLogf(ctx, "Command have duplicate prefix, message %q", msg)
				return nil, errors.Errorf("duplicate prefix %q found", prefix)
			}
			parsed[prefix] = line[len(prefix):]
		}
	}
	if checkMatch && len(parsed) != len(prefixes) {
		return nil, errors.Errorf("missing attribute/prefix, message %q", msg)
	}
	return parsed, nil
}

// parseNonsensitiveStatusInfo tries to parse the output of NonsensitiveStatus from msg, if checkStatus is true, then we'll verify that the output of the command contains a success message.
func parseNonsensitiveStatusInfo(ctx context.Context, checkStatus bool, msg string) (info *NonsensitiveStatusInfo, returnedError error) {
	const (
		IsEnablePrefix                = "  is_enabled: "
		IsOwnedPrefix                 = "  is_owned: "
		IsOwnerPasswordPresentPrefix  = "  is_owner_password_present: "
		HasResetLockPermissionsPrefix = "  has_reset_lock_permissions: "
		StatusPrefix                  = "  status: "
	)
	prefixes := []string{
		IsEnablePrefix,
		IsOwnedPrefix,
		IsOwnerPasswordPresentPrefix,
		HasResetLockPermissionsPrefix,
		StatusPrefix,
	}

	parsed, err := parseStringMap(ctx, msg, checkStatus, prefixes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse string map")
	}

	if checkStatus {
		// We need to check the status.
		if parsed[StatusPrefix] != tpmManagerStatusSuccessMessage {
			return nil, errors.Errorf("incorrect status %q from NonsensitiveStatusInfo", parsed[StatusPrefix])
		}
	}

	isEnabled := false
	if _, err := fmt.Sscanf(parsed[IsEnablePrefix], "%t", &isEnabled); err != nil {
		return nil, errors.Wrapf(err, "isEnabled doesn't start with a valid boolean %q", parsed[IsEnablePrefix])
	}
	isOwned := false
	if _, err := fmt.Sscanf(parsed[IsOwnedPrefix], "%t", &isOwned); err != nil {
		return nil, errors.Wrapf(err, "isOwned doesn't start with a valid boolean %q", parsed[IsOwnedPrefix])
	}
	ownerPass := false
	if _, err := fmt.Sscanf(parsed[IsOwnerPasswordPresentPrefix], "%t", &ownerPass); err != nil {
		return nil, errors.Wrapf(err, "ownerPass doesn't start with a valid boolean %q", parsed[IsOwnerPasswordPresentPrefix])
	}
	lockPerm := false
	if _, err := fmt.Sscanf(parsed[HasResetLockPermissionsPrefix], "%t", &lockPerm); err != nil {
		return nil, errors.Wrapf(err, "lockPerm doesn't start with a valid boolean %q", parsed[HasResetLockPermissionsPrefix])
	}

	return &NonsensitiveStatusInfo{
		IsEnabled:               isEnabled,
		IsOwned:                 isOwned,
		IsOwnerPasswordPresent:  ownerPass,
		HasResetLockPermissions: lockPerm,
	}, nil
}

// GetNonsensitiveStatus retrieves the NonsensitiveStatusInfo.
func (u *UtilityTpmManagerBinary) GetNonsensitiveStatus(ctx context.Context) (info *NonsensitiveStatusInfo, returnedError error) {
	binaryMsg, err := u.binary.NonsensitiveStatus(ctx)

	// Convert msg first because it's still used when there's an error.
	msg := string(binaryMsg)

	if err != nil {
		return nil, errors.Wrapf(err, "calling NonsensitiveStatus failed with message %q", msg)
	}

	// Now try to parse everything.
	return parseNonsensitiveStatusInfo(ctx, true, msg)
}

// DAInfo contains the dictionary attack related information.
type DAInfo struct {
	// Counter is the dictionary attack lockout counter.
	Counter int

	// Threshold is the dictionary attack lockout threshold.
	Threshold int

	// InEffect indicates if dictionary attack lockout is in effect.
	InEffect bool

	// Remaining is the seconds remaining until we can reset the lockout.
	Remaining int
}

// parseDAInfo tries to parse the output of GetDAInfo from msg, if checkStatus is true, then we'll verify that the output of the command contains a success message.
func parseDAInfo(ctx context.Context, checkStatus bool, msg string) (info *DAInfo, returnedError error) {
	const (
		CounterPrefix   = "  dictionary_attack_counter: "
		ThresholdPrefix = "  dictionary_attack_threshold: "
		InEffectPrefix  = "  dictionary_attack_lockout_in_effect: "
		RemainingPrefix = "  dictionary_attack_lockout_seconds_remaining: "
		StatusPrefix    = "  status: "
	)
	prefixes := []string{CounterPrefix, ThresholdPrefix, InEffectPrefix, RemainingPrefix, StatusPrefix}

	parsed, err := parseStringMap(ctx, msg, checkStatus, prefixes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse string map")
	}

	if checkStatus {
		// We need to check the status.
		if parsed[StatusPrefix] != tpmManagerStatusSuccessMessage {
			return nil, errors.Errorf("incorrect status %q from GetDAInfo", parsed[StatusPrefix])
		}
	}

	counter := -1
	if _, err := fmt.Sscanf(parsed[CounterPrefix], "%d", &counter); err != nil {
		return nil, errors.Wrapf(err, "counter doesn't start with a valid integer %q", parsed[CounterPrefix])
	}

	threshold := -1
	if _, err := fmt.Sscanf(parsed[ThresholdPrefix], "%d", &threshold); err != nil {
		return nil, errors.Wrapf(err, "threshold doesn't start with a valid integer %q", parsed[ThresholdPrefix])
	}

	inEffect := false
	if _, err := fmt.Sscanf(parsed[InEffectPrefix], "%t", &inEffect); err != nil {
		return nil, errors.Wrapf(err, "in effect doesn't start with a valid boolean %q", parsed[InEffectPrefix])
	}

	remaining := -1
	if _, err := fmt.Sscanf(parsed[RemainingPrefix], "%d", &remaining); err != nil {
		return nil, errors.Wrapf(err, "remaining doesn't start with a valid integer %q", parsed[RemainingPrefix])
	}

	return &DAInfo{Counter: counter, Threshold: threshold, InEffect: inEffect, Remaining: remaining}, nil
}

// GetDAInfo retrieves the dictionary attack counter, threshold, if lockout is in effect and seconds remaining. The returned err is nil iff the operation is successful.
func (u *UtilityTpmManagerBinary) GetDAInfo(ctx context.Context) (info *DAInfo, returnedError error) {
	binaryMsg, err := u.binary.GetDAInfo(ctx)

	// Convert msg first because it's still used when there's an error.
	msg := string(binaryMsg)

	if err != nil {
		return nil, errors.Wrapf(err, "calling GetDAInfo failed with message %q", msg)
	}

	// Now try to parse everything.
	return parseDAInfo(ctx, true, msg)
}

// ResetDALock resets the dictionary attack lockout.
func (u *UtilityTpmManagerBinary) ResetDALock(ctx context.Context) (string, error) {
	binaryMsg, err := u.binary.ResetDALock(ctx)

	// Convert msg first because it's still used when there's an error.
	msg := string(binaryMsg)

	// Check if the command succeeds.
	if err != nil {
		// Command failed.
		return msg, errors.Wrapf(err, "calling ResetDALock failed with message %q", msg)
	}

	// Check if the message make sense.
	if !strings.Contains(msg, tpmManagerStatusSuccessMessage) {
		return msg, errors.Errorf("calling ResetDALock failed due to unexpected stdout %q", msg)
	}

	return msg, nil
}

// DropResetLockPermissions drops the reset lock permissions and return a callback to restore the permissions.
func (u *UtilityTpmManagerBinary) DropResetLockPermissions(ctx context.Context) (func() error, error) {
	rawData, err := u.binary.GetLocalTPMData(ctx)
	if err != nil {
		return func() error { return nil }, errors.Wrap(err, "failed to get local TPM data")
	}

	var data tmpb.LocalData
	if err := proto.Unmarshal(rawData, &data); err != nil {
		return func() error { return nil }, errors.Wrap(err, "failed to unmarshal local TPM data")
	}

	data.OwnerPassword = []byte{}
	data.LockoutPassword = []byte{}
	data.OwnerDelegate = &tmpb.AuthDelegate{}

	newData, err := proto.Marshal(&data)
	if err != nil {
		return func() error { return nil }, errors.Wrap(err, "failed to marshal local TPM data")
	}

	if err := u.binary.SetLocalTPMData(ctx, newData); err != nil {
		return func() error { return nil }, errors.Wrap(err, "failed to get local TPM data")
	}

	return func() error {
		return u.binary.SetLocalTPMData(ctx, rawData)
	}, nil
}
