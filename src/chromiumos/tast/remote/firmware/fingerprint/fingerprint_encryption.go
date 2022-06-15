// Copyright 2022 The ChromiumOS Authors.
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

// Set checks if the given flags are set.
func (f EncryptionStatusFlags) Set(flags EncryptionStatusFlags) bool {
	return (f & flags) == flags
}

// unmarshalEctoolFlags unmarshals part of the ectool output into a EncryptionStatusFlags.
func unmarshalEctoolFlags(data string) (EncryptionStatusFlags, error) {
	flags, err := strconv.ParseUint(data, 0, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert encryption status flags (%s) to int", data)
	}
	return EncryptionStatusFlags(flags), nil
}

// EncryptionStatus hold the state of encryption engine from an FPMCU.
type EncryptionStatus struct {
	Current EncryptionStatusFlags
	Valid   EncryptionStatusFlags
}

// TPMSeedSet is used to obtain information about TPM seed presence as
// reported by the FPMCU.
func (e *EncryptionStatus) TPMSeedSet() bool {
	return e.Current.Set(EncryptionStatusTPMSeedSet)
}

// unmarshalEctoolEncryptionStatus unmarshals part of the ectool output into
// a EncryptionStatus.
func unmarshalEctoolEncryptionStatus(data string) (EncryptionStatus, error) {
	reCurrent := regexp.MustCompile(`FPMCU encryption status\:\s+(0x[[:xdigit:]]+)`)
	reValid := regexp.MustCompile(`Valid flags\:\s+(0x[[:xdigit:]]+)`)

	result := reCurrent.FindStringSubmatch(data)
	if result == nil || len(result) != 2 {
		return EncryptionStatus{}, errors.Errorf("can't find current encryption status flags in %q", data)
	}
	current, err := unmarshalEctoolFlags(result[1])
	if err != nil {
		return EncryptionStatus{}, errors.Wrap(err, "failed to unmarshal current flags")
	}

	result = reValid.FindStringSubmatch(data)
	if result == nil || len(result) != 2 {
		return EncryptionStatus{}, errors.Errorf("can't find valid encryption status flags in %q", data)
	}
	valid, err := unmarshalEctoolFlags(result[1])
	if err != nil {
		return EncryptionStatus{}, errors.Wrap(err, "failed to unmarshal valid flags")
	}

	return EncryptionStatus{current, valid}, nil
}

// GetEncryptionStatus is used to obtain actual encryption engine state
// as reported by the FPMCU using the 'ectool --name=cros_fp fpencstatus'
// command.
func GetEncryptionStatus(ctx context.Context, d *dut.DUT) (EncryptionStatus, error) {
	cmd := firmware.NewECTool(d, firmware.ECToolNameFingerprint).Command(ctx, "fpencstatus")
	bytes, err := cmd.Output()
	if err != nil {
		return EncryptionStatus{}, errors.Wrap(err, "failed to get FPMCU encryption engine state")
	}
	output := string(bytes)
	return unmarshalEctoolEncryptionStatus(output)
}
