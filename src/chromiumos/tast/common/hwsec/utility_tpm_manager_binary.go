// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
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

// checkNVRAMCommandAndReturn is a simple helper that checks if binaryMsg returned from NVRAM related command is successfully, and returns the corresponding message and error.
func checkNVRAMCommandAndReturn(ctx context.Context, binaryMsg []byte, err error, command string) (string, error) {
	// Convert msg first because it's still used when there's an error.
	msg := ""
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}

	// Check if the command succeeds.
	if err != nil {
		// Command failed.
		return msg, errors.Wrapf(err, "calling %s failed with message %q", command, msg)
	}

	// Check if the message make sense.
	if !strings.Contains(msg, tpmManagerNVRAMSuccessMessage) {
		return msg, errors.Errorf("calling %s failed due to unexpected stdout %q", command, msg)
	}

	return msg, nil
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

// ResetDALock resets the dictionary attack lockout.
func (u *UtilityTpmManagerBinary) ResetDALock(ctx context.Context) (string, error) {
	binaryMsg, err := u.binary.ResetDALock(ctx)

	// Convert msg first because it's still used when there's an error.
	msg := ""
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}

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
