// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

// EncryptionStatusFlags represents a state of FPMCU encryption engine.
type EncryptionStatusFlags uint64

// Individual encryption status flags.
const (
	EncryptionStatusTPMSeedSet EncryptionStatusFlags = 0x1
)

// IsSet checks if the given flags are set.
func (f EncryptionStatusFlags) IsSet(flags EncryptionStatusFlags) bool {
	return (f & flags) == flags
}

// UnmarshalerEctool unmarshals part of the ectool output into a EncryptionStatusFlags.
func (f *EncryptionStatusFlags) UnmarshalerEctool(data []byte) error {
	flagString := string(data)
	flags, err := strconv.ParseUint(flagString, 0, 32)
	if err != nil {
		return errors.Wrapf(err, "failed to convert encryption status flags (%s) to int", flagString)
	}
	*f = EncryptionStatusFlags(flags)
	return nil
}

// EncryptionStatus hold the state of encryption engine from an FPMCU.
type EncryptionStatus struct {
	Current EncryptionStatusFlags
	Valid   EncryptionStatusFlags
}

// IsTPMSeedSet is used to obtain information about TPM seed presence as
// reported by the FPMCU.
func (e *EncryptionStatus) IsTPMSeedSet() bool {
	return e.Current.IsSet(EncryptionStatusTPMSeedSet)
}

// UnmarshalerEctool unmarshals part of the ectool output into a EncryptionStatus.
func (e *EncryptionStatus) UnmarshalerEctool(data []byte) error {
	dataStr := string(data)
	reCurrent := regexp.MustCompile(`FPMCU encryption status\:\s+(0x[[:xdigit:]]+)`)
	reValid := regexp.MustCompile(`Valid flags\:\s+(0x[[:xdigit:]]+)`)

	var es EncryptionStatus
	var flags EncryptionStatusFlags

	result := reCurrent.FindStringSubmatch(dataStr)
	if result == nil || len(result) != 2 {
		return errors.Errorf("can't find current encryption status flags in %q", dataStr)
	}
	if err := flags.UnmarshalerEctool([]byte(result[1])); err != nil {
		return errors.Wrap(err, "failed to unmarshal current flags")
	}
	es.Current = flags

	result = reValid.FindStringSubmatch(dataStr)
	if result == nil || len(result) != 2 {
		return errors.Errorf("can't find valid encryption status flags in %q", dataStr)
	}
	if err := flags.UnmarshalerEctool([]byte(result[1])); err != nil {
		return errors.Wrap(err, "failed to unmarshal valid flags")
	}
	es.Valid = flags

	*e = es
	return nil
}

// GetEncryptionStatus is used to obtain actual encryption engine state
// as reported by the FPMCU using the 'ectool --name=cros_fp fpencstatus'
// command.
func GetEncryptionStatus(ctx context.Context, d *dut.DUT) (EncryptionStatus, error) {
	var e EncryptionStatus
	cmd := firmware.NewECTool(d, firmware.ECToolNameFingerprint).Command(ctx, "fpencstatus")
	bytes, err := cmd.Output()
	if err != nil {
		return e, errors.Wrap(err, "failed to get FPMCU encryption engine state")
	}
	return e, e.UnmarshalerEctool(bytes)
}
