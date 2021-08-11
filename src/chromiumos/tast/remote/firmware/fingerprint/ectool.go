// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// FWImageType is the type of firmware (RO or RW).
type FWImageType string

// These are the possible values of FWImageType.
const (
	ImageTypeRO FWImageType = "RO"
	ImageTypeRW FWImageType = "RW"
)

const (
	ectoolROVersion = "RO version"
	ectoolRWVersion = "RW version"
)

// EctoolCommand constructs an "ectool" command for the FPMCU.
func EctoolCommand(ctx context.Context, d *dut.DUT, args ...string) *ssh.Cmd {
	cmd := firmware.NewECTool(d, firmware.ECToolNameFingerprint).Command(args...)
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd.Args))
	return cmd
}

// RollbackState is the state of the anti-rollback block.
type RollbackState struct {
	BlockID    int
	MinVersion int
	RWVersion  int
}

// UnmarshalerEctool unmarshals part of ectool's output into a RollbackState.
func (r *RollbackState) UnmarshalerEctool(data []byte) error {
	rollbackInfoMap := parseColonDelimitedOutput(string(data))

	var state RollbackState
	blockID, err := strconv.Atoi(rollbackInfoMap["Rollback block id"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback block id")
	}
	state.BlockID = blockID

	minVersion, err := strconv.Atoi(rollbackInfoMap["Rollback min version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback min version")
	}
	state.MinVersion = minVersion

	rwVersion, err := strconv.Atoi(rollbackInfoMap["RW rollback version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert RW rollback version")
	}
	state.RWVersion = rwVersion

	*r = state
	return nil
}

// IsEntropySet checks that entropy has already been set based on the block ID.
//
// If the block ID is greater than 0, there is a very good chance that entropy
// has been added. This is the same way that biod/bio_wash checks if entropy has
// been set. That being said, this method can be fooled if some test simply
// increments the anti-rollback version from a fresh flashing.
func (r *RollbackState) IsEntropySet() bool {
	return r.BlockID > 0
}

// IsAntiRollbackSet checks if version anti-rollback has been enabled.
//
// We currently do not have a minimum version number, thus this function
// indicates if we are not in the normal rollback state.
func (r *RollbackState) IsAntiRollbackSet() bool {
	return r.MinVersion != 0 || r.RWVersion != 0
}

// RollbackInfo returns the rollbackinfo of the fingerprint MCU.
func RollbackInfo(ctx context.Context, d *dut.DUT) (RollbackState, error) {
	cmd := []string{"ectool", "--name=cros_fp", "rollbackinfo"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd))
	out, err := d.Conn().Command(cmd[0], cmd[1:]...).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return RollbackState{}, errors.Wrap(err, "failed to query FPMCU rollbackinfo")
	}

	var state RollbackState
	err = state.UnmarshalerEctool(out)
	return state, err
}

// AddEntropy adds entropy to the fingerprint MCU.
func AddEntropy(ctx context.Context, d *dut.DUT, reset bool) error {
	args := []string{"addentropy"}
	if reset {
		args = append(args, "reset")
	}
	return EctoolCommand(ctx, d, args[0:]...).Run(ctx)
}

// RebootFpmcu reboots the fingerprint MCU. It does not reboot the AP.
func RebootFpmcu(ctx context.Context, d *dut.DUT, bootTo FWImageType) error {
	testing.ContextLog(ctx, "Rebooting FPMCU")
	// This command returns error even on success, so ignore error. b/116396469
	_ = EctoolCommand(ctx, d, "reboot_ec").Run(ctx)
	if bootTo == ImageTypeRO {
		testing.Sleep(ctx, 500*time.Millisecond)
		err := EctoolCommand(ctx, d, "rwsigaction", "abort").Run(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to abort rwsig")
		}
	}

	if err := WaitForRunningFirmwareImage(ctx, d, bootTo); err != nil {
		return errors.Wrapf(err, "failed to boot to %q image", bootTo)
	}

	// Double check we are still in the expected image.
	firmwareCopy, err := RunningFirmwareCopy(ctx, d)
	if err != nil {
		return err
	}
	if firmwareCopy != bootTo {
		return errors.Errorf("FPMCU booted to %q, expected %q", firmwareCopy, bootTo)
	}
	return nil
}

// RunningFirmwareCopy returns the firmware copy on FPMCU (RO or RW).
func RunningFirmwareCopy(ctx context.Context, d *dut.DUT) (FWImageType, error) {
	out, err := EctoolCommand(ctx, d, "version").Output(ctx)
	if err != nil {
		return FWImageType(""), errors.Wrap(err, "failed to query FPMCU version")
	}
	versionInfoMap := parseColonDelimitedOutput(string(out))
	firmwareCopy := versionInfoMap["Firmware copy"]
	if firmwareCopy != string(ImageTypeRO) && firmwareCopy != string(ImageTypeRW) {
		return FWImageType(""), errors.New("cannot find firmware copy string")
	}
	return FWImageType(firmwareCopy), nil
}

// CheckFirmwareIsFunctional checks that the AP can talk to the FPMCU and get the version.
func CheckFirmwareIsFunctional(ctx context.Context, d *dut.DUT) ([]byte, error) {
	testing.ContextLog(ctx, "Checking firmware is functional")
	return EctoolCommand(ctx, d, "version").Output(ctx, ssh.DumpLogOnError)
}

// WaitForRunningFirmwareImage waits for the requested image to boot.
func WaitForRunningFirmwareImage(ctx context.Context, d *dut.DUT, image FWImageType) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		firmwareCopy, err := RunningFirmwareCopy(ctx, d)
		if err != nil {
			return err
		}
		if firmwareCopy != image {
			return errors.Errorf("FPMCU booted to %q, expected %q", firmwareCopy, image)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})
}

// CheckRunningFirmwareCopy validates that image is the running FPMCU firmware copy
// and returns an error if that is not the case.
func CheckRunningFirmwareCopy(ctx context.Context, d *dut.DUT, image FWImageType) error {
	runningImage, err := RunningFirmwareCopy(ctx, d)
	if err != nil {
		return err
	}
	if runningImage != image {
		return errors.Errorf("failed to validate the firmware image, got %q, want %q", runningImage, image)
	}
	return nil
}

// runningFirmwareVersion returns the current RO or RW firmware version on the FPMCU.
func runningFirmwareVersion(ctx context.Context, d *dut.DUT, image FWImageType) (string, error) {
	out, err := EctoolCommand(ctx, d, "version").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to query FPMCU version")
	}
	versionInfoMap := parseColonDelimitedOutput(string(out))
	switch image {
	case ImageTypeRW:
		return versionInfoMap[ectoolRWVersion], nil
	case ImageTypeRO:
		return versionInfoMap[ectoolROVersion], nil
	default:
		return "", errors.Errorf("unrecognized image type: %q", image)
	}
}

func rawFPFrameCommand(ctx context.Context, d *dut.DUT) *ssh.Cmd {
	return EctoolCommand(ctx, d, "fpframe", "raw")
}

// parseColonDelimitedOutput parses colon delimited information to a map.
func parseColonDelimitedOutput(output string) map[string]string {
	ret := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		// Note that the ectool version build info line uses ':'s as time of
		// date delimiters.
		splits := strings.SplitN(line, ":", 2)
		if len(splits) != 2 {
			continue
		}
		ret[strings.TrimSpace(splits[0])] = strings.TrimSpace(splits[1])
	}
	return ret
}
