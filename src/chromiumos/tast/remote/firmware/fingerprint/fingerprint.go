// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's fingerprint MCU.

package fingerprint

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// nocturne and nami are special cases and have "_fp" appended.
	// Newer FPMCUs have unique names.
	// See go/cros-fingerprint-firmware-branching-and-signing.
	fingerprintBoardNameSuffix  = "_fp"
	fingerprintFirmwarePathBase = "/opt/google/biod/fw/"
	// WaitForBiodToStartTimeout is the time to wait for biod to start.
	WaitForBiodToStartTimeout = 30 * time.Second

	// Types of signing keys.
	keyTypeDev   = "dev"
	keyTypePreMp = "premp"
	keyTypeMp    = "mp"

	// EC board names for FPMCUs.
	fpBoardNameBloonchipper = "bloonchipper"
	fpBoardNameDartmonkey   = "dartmonkey"
	fpBoardNameNocturne     = "nocturne_fp"
	fpBoardNameNami         = "nami_fp"

	// Elements of firmware information.
	fwInfoTypeSha256sum = "sha256sum"
	fwInfoTypeRoVersion = "ro_version"
	fwInfoTypeRwVersion = "rw_version"
	fwInfoTypeKeyID     = "key_id"
)

// Map from signing key ID to type of signing key.
var keyIDMap = map[string]string{
	// bloonchipper
	"61382804da86b4156d666cc9a976088f8b647d44": keyTypeDev,
	"07b1af57220c196e363e68d73a5966047c77011e": keyTypePreMp,
	"1c590ef36399f6a2b2ef87079c135b69ef89eb60": keyTypeMp,

	// dartmonkey
	"257a0aa3ac9e81aa4bc3aabdb6d3d079117c5799": keyTypeMp,

	// nocturne
	"8a8fc039a9463271995392f079b83ce33832d07d": keyTypeDev,
	"6f38c866182bd9bf7a4462c06ac04fa6a0074351": keyTypeMp,
	"f6f7d96c48bd154dbae7e3fe3a3b4c6268a10934": keyTypePreMp,

	// nami
	"754aea623d69975a22998f7b97315dd53115d723": keyTypePreMp,
	"35486c0090ca390408f1fbbf2a182966084fe2f8": keyTypeMp,
}

// Map of attributes for a given board's various firmware file releases.
// Two purposes:
//   1) Documents the exact versions and keys used for a given firmware file.
//   2) Used to verify that files that end up in the build (and therefore
//      what we release) is exactly what we expect.
var firmwareVersionMap = map[string]map[string]map[string]string{
	fpBoardNameBloonchipper: map[string]map[string]string{
		"bloonchipper_v2.0.4277-9f652bb3-RO_v2.0.7314-3dfc5ff6-RW.bin": map[string]string{
			fwInfoTypeSha256sum: "2bac89c16ad71986fe37ed651fe7dd6d5a3d039678d4a5f1d03c5a65a9f3bc3c",
			fwInfoTypeRoVersion: "bloonchipper_v2.0.4277-9f652bb3",
			fwInfoTypeRwVersion: "bloonchipper_v2.0.7314-3dfc5ff6",
			fwInfoTypeKeyID:     "1c590ef36399f6a2b2ef87079c135b69ef89eb60",
		},
		"bloonchipper_v2.0.5938-197506c1-RO_v2.0.7314-3dfc5ff6-RW.bin": map[string]string{
			fwInfoTypeSha256sum: "50ddcad558e1ded476a209946cabcddd6d9c1033890f1661d7ba8c183aa625ab",
			fwInfoTypeRoVersion: "bloonchipper_v2.0.5938-197506c1",
			fwInfoTypeRwVersion: "bloonchipper_v2.0.7314-3dfc5ff6",
			fwInfoTypeKeyID:     "1c590ef36399f6a2b2ef87079c135b69ef89eb60",
		},
	},
	fpBoardNameNocturne: map[string]map[string]string{
		"nocturne_fp_v2.2.64-58cf5974e-RO_v2.0.7304-441100b93-RW.bin": map[string]string{
			fwInfoTypeSha256sum: "569a191bd2ed25ce89b296f0ab8cd2ed567dbf6a8df3f6b3f82ad58c786d79a9",
			fwInfoTypeRoVersion: "nocturne_fp_v2.2.64-58cf5974e",
			fwInfoTypeRwVersion: "nocturne_fp_v2.0.7304-441100b93",
			fwInfoTypeKeyID:     "6f38c866182bd9bf7a4462c06ac04fa6a0074351",
		},
	},
	fpBoardNameNami: map[string]map[string]string{
		"nami_fp_v2.2.144-7a08e07eb-RO_v2.0.7304-441100b93-RW.bin": map[string]string{
			fwInfoTypeSha256sum: "e7b23f5e585c47d24fe3696139b48c0bac8c43b025669f74aafbff4aa9cbbebd",
			fwInfoTypeRoVersion: "nami_fp_v2.2.144-7a08e07eb",
			fwInfoTypeRwVersion: "nami_fp_v2.0.7304-441100b93",
			fwInfoTypeKeyID:     "35486c0090ca390408f1fbbf2a182966084fe2f8",
		},
	},
	fpBoardNameDartmonkey: map[string]map[string]string{
		"dartmonkey_v2.0.2887-311310808-RO_v2.0.7304-441100b93-RW.bin": map[string]string{
			fwInfoTypeSha256sum: "5127137655b4b13d7a86ba897b08a9957d36b74afb97558496c6fba98e808b7b",
			fwInfoTypeRoVersion: "dartmonkey_v2.0.2887-311310808",
			fwInfoTypeRwVersion: "dartmonkey_v2.0.7304-441100b93",
			fwInfoTypeKeyID:     "257a0aa3ac9e81aa4bc3aabdb6d3d079117c5799",
		},
	},
}

// getKeyType returns the key "type" for a given "key id".
func getKeyType(keyID string) (string, error) {
	keyType, ok := keyIDMap[keyID]
	if !ok {
		return "", errors.Errorf("unable to get key type for key id: %s", keyID)
	}
	return keyType, nil
}

// getExpectedFwInfo eturns expected firmware info for a given firmware file name.
func getExpectedFwInfo(fpBoard, buildFwFile, infoType string) (string, error) {
	boardExpectedFwInfo, ok := firmwareVersionMap[fpBoard]
	if !ok {
		return "", errors.Errorf("unable to get firmware info for board: %s", fpBoard)
	}
	expectedFwInfo, ok := boardExpectedFwInfo[filepath.Base(buildFwFile)]
	if !ok {
		return "", errors.Errorf("unable to get firmware info for file: %s", buildFwFile)
	}
	ret, ok := expectedFwInfo[infoType]
	if !ok {
		return "", errors.Errorf("unable to get firmware info type: %s", infoType)
	}
	return ret, nil
}

// ValidateBuildFwFile checks that all attributes in the given firmware file match their expected values.
func ValidateBuildFwFile(ctx context.Context, d *dut.DUT, fpBoard, buildFwFile string) error {
	// Check hash on device.
	actualHash, err := calculateSha256sum(ctx, d, buildFwFile)
	if err != nil {
		return err
	}
	expectedHash, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeSha256sum)
	if err != nil {
		return err
	}
	if actualHash != expectedHash {
		return errors.Errorf("sha256 sum %s does not match expected %s", actualHash, expectedHash)
	}

	// Check signing key ID.
	actualKeyID, err := readFirmwareKeyID(ctx, d, buildFwFile)
	if err != nil {
		return err
	}
	expectedKeyID, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeKeyID)
	if err != nil {
		return err
	}
	if actualKeyID != expectedKeyID {
		return errors.Errorf("key id %s does not match expected %s", actualKeyID, expectedKeyID)
	}

	// Check the signing key type is allowed.
	keyType, err := getKeyType(actualKeyID)
	if err != nil {
		return err
	}
	if keyType != keyTypePreMp && keyType != keyTypeMp {
		return errors.Errorf("key type %s is not allowed", keyType)
	}

	// Check RO version
	actualRoVersion, err := readFmapSection(ctx, d, buildFwFile, "RO_FRID")
	if err != nil {
		return err
	}
	expectedRoVersion, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeRoVersion)
	if err != nil {
		return err
	}
	if actualRoVersion != expectedRoVersion {
		return errors.Errorf("RO version %s does not match expected %s", actualRoVersion, expectedRoVersion)
	}

	// Check RW version
	actualRwVersion, err := readFmapSection(ctx, d, buildFwFile, "RW_FWID")
	if err != nil {
		return err
	}
	expectedRwVersion, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeRwVersion)
	if err != nil {
		return err
	}
	if actualRwVersion != expectedRwVersion {
		return errors.Errorf("RW version %s does not match expected %s", actualRwVersion, expectedRwVersion)
	}

	testing.ContextLog(ctx, "Validated build firmware metadata")
	return nil
}

// readFmapSection reads a section (e.g. RO_FRID) from a firmware file on device.
func readFmapSection(ctx context.Context, d *dut.DUT, buildFwFile, section string) (string, error) {
	// Prepare a temporary file because dump_map only writes the
	// value read from a section to a file (will not just print it to
	// stdout).
	tempdir, err := d.Command("mktemp", "-d", "/tmp/fingerprint_dump_fmap_XXXXXX").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create remote temp directory")
	}
	tempdirPath := strings.TrimSpace(string(tempdir))
	defer d.Command("rm", "-r", tempdirPath).Run(ctx)

	outputPath := filepath.Join(tempdirPath, section)
	if err := d.Command("dump_fmap", "-x", buildFwFile, fmt.Sprintf("%s:%s", section, outputPath)).Run(ctx); err != nil {
		return "", errors.Wrap(err, "failed to run dump_fmap")
	}

	out, err := d.Command("cat", outputPath).Output(ctx)
	// dump_fmap writes NULL characters at the end.
	return strings.Trim(string(out), "\x00"), err
}

// readFirmwareKeyID reads the key id of a firmware file on device.
func readFirmwareKeyID(ctx context.Context, d *dut.DUT, buildFwFile string) (string, error) {
	out, err := d.Command("futility", "show", buildFwFile).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to run futility on device")
	}
	parsed := parseColonDelimitedOutput(string(out))
	keyID, ok := parsed["ID"]
	if !ok {
		return "", errors.Errorf("failed to find key ID for %s", buildFwFile)
	}
	return keyID, nil
}

// calculateSha256sum calculates the sha256sum of a file on device.
func calculateSha256sum(ctx context.Context, d *dut.DUT, buildFwFile string) (string, error) {
	out, err := d.Command("sha256sum", buildFwFile).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to calculate sha256sum on device")
	}
	return strings.Split(string(out), " ")[0], nil
}

// boardFromCrosConfig returns the fingerprint board name from cros_config.
func boardFromCrosConfig(ctx context.Context, d *dut.DUT) (string, error) {
	out, err := d.Command("cros_config", "/fingerprint", "board").Output(ctx)
	return string(out), err
}

// Board returns the name of the fingerprint EC on the DUT
func Board(ctx context.Context, d *dut.DUT) (string, error) {
	// For devices that don't have unibuild support (which is required to
	// use cros_config).
	// TODO(https://crbug.com/1030862): remove when nocturne has cros_config
	// support.
	board, err := reporters.New(d).Board(ctx)
	if err != nil {
		return "", err
	}
	if board == "nocturne" {
		return board + fingerprintBoardNameSuffix, nil
	}

	// Use cros_config to get fingerprint board.
	return boardFromCrosConfig(ctx, d)
}

// FirmwarePath returns the path to the fingerprint firmware file on device.
func FirmwarePath(ctx context.Context, d *dut.DUT, fpBoard string) (string, error) {
	cmd := fmt.Sprintf("ls %s%s*.bin", fingerprintFirmwarePathBase, fpBoard)
	out, err := d.Command("bash", "-c", cmd).Output(ctx)
	if err != nil {
		return "", err
	}
	outStr := strings.TrimSpace(string(out))
	if strings.Contains(outStr, "\n") {
		return "", errors.Errorf("found multiple firmware files for %q: %s", fpBoard, outStr)
	}
	return outStr, nil
}

// FlashFirmware flashes the original fingerprint firmware in rootfs.
func FlashFirmware(ctx context.Context, d *dut.DUT) error {
	fpBoard, err := Board(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get fp board")
	}
	testing.ContextLogf(ctx, "fp board name: %q", fpBoard)

	fpFirmwarePath, err := FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		return errors.Wrap(err, "failed to get fp firmware path")
	}
	flashCmd := []string{"flash_fp_mcu", fpFirmwarePath}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if err := d.Command(flashCmd[0], flashCmd[1:]...).Run(ctx); err != nil {
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	return nil
}

// InitializeEntropy initializes the anti-rollback block in RO firmware.
func InitializeEntropy(ctx context.Context, d *dut.DUT) error {
	if err := d.Command("bio_wash", "--factory_init").Run(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize entropy")
	}
	return nil
}

// CheckFirmwareIsFunctional checks that the AP can talk to the FPMCU and get the version.
func CheckFirmwareIsFunctional(ctx context.Context, d *dut.DUT) ([]byte, error) {
	testing.ContextLog(ctx, "Checking firmware is functional")
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
	return d.Command(versionCmd[0], versionCmd[1:]...).Output(ctx)
}

// InitializeKnownState checks that the AP can talk to FPMCU. If not, it flashes the FPMCU.
func InitializeKnownState(ctx context.Context, d *dut.DUT, outdir string, pxy *servo.Proxy) error {
	if out, err := CheckFirmwareIsFunctional(ctx, d); err == nil {
		versionOutputFile := "cros_fp_version.txt"
		testing.ContextLogf(ctx, "Writing FP firmware version to %s", versionOutputFile)
		if err := ioutil.WriteFile(filepath.Join(outdir, versionOutputFile), out, 0644); err != nil {
			// This is a nonfatal error that shouldn't kill the test.
			testing.ContextLog(ctx, "Failed to write FP firmware version to file: ", err)
		}
	} else {
		testing.ContextLogf(ctx, "FPMCU firmware is not functional (error: %v). Trying re-flashing FP firmware", err)
		if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_off"); err != nil {
			return errors.Wrap(err, "failed to disable HW write protect")
		}
		if err := FlashFirmware(ctx, d); err != nil {
			return errors.Wrap(err, "failed to flash FP firmware")
		}
		testing.ContextLog(ctx, "Flashed FP firmware, now initializing the entropy")
		if err := InitializeEntropy(ctx, d); err != nil {
			return err
		}
		testing.ContextLog(ctx, "Entropy initialized, now rebooting to get seed")
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "failed to reboot DUT")
		}
	}
	// Enable hardware write protect so that we are in the same state as the end user.
	if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_on"); err != nil {
		return errors.Wrap(err, "failed to enable HW write protect")
	}
	return nil
}

// InitializeHWAndSWWriteProtect ensures hardware and software write protect are initialized as requested.
func InitializeHWAndSWWriteProtect(ctx context.Context, d *dut.DUT, pxy *servo.Proxy, enableHWWP, enableSWWP bool) error {
	testing.ContextLogf(ctx, "Initializing HW WP to %t, SW WP to %t", enableHWWP, enableSWWP)
	// HW write protect must be disabled to disable SW write protect.
	hwWPArg := "force_off"
	if !enableSWWP {
		if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, hwWPArg); err != nil {
			return errors.Wrap(err, "failed to disable HW write protect")
		}
	}

	swWPArg := "disable"
	if enableSWWP {
		swWPArg = "enable"
	}
	// This command can return error even on success, so ignore error.
	_, _ = d.Command("ectool", "--name=cros_fp", "flashprotect", swWPArg).Output(ctx)
	testing.Sleep(ctx, 2*time.Second)
	if err := RebootFpmcu(ctx, d, "RW"); err != nil {
		return err
	}

	if enableHWWP {
		hwWPArg = "force_on"
	}
	// Don't use SetStringAndCheck because the state can be "on" after we set "force_on".
	if err := pxy.Servo().SetString(ctx, servo.FWWPState, hwWPArg); err != nil {
		return errors.Wrapf(err, "failed to set HW write protect to %q", hwWPArg)
	}
	// TODO(yichengli): Check the correct flags, which is different for different chips.
	return nil
}

// RebootFpmcu reboots the fingerprint MCU. It does not reboot the AP.
func RebootFpmcu(ctx context.Context, d *dut.DUT, bootTo string) error {
	testing.ContextLog(ctx, "Rebooting FPMCU")
	// This command returns error even on success, so ignore error. b/116396469
	_ = d.Command("ectool", "--name=cros_fp", "reboot_ec").Run(ctx)
	if bootTo == "RO" {
		testing.Sleep(ctx, 500*time.Millisecond)
		err := d.Command("ectool", "--name=cros_fp", "rwsigaction", "abort").Run(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to abort rwsig")
		}
	}
	err := testing.Poll(ctx, func(ctx context.Context) error {
		firmwareCopy, err := RunningFirmwareCopy(ctx, d)
		if err != nil {
			return err
		}
		if firmwareCopy != bootTo {
			return errors.Errorf("FPMCU booted to %q, expected %q", firmwareCopy, bootTo)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})

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
func RunningFirmwareCopy(ctx context.Context, d *dut.DUT) (string, error) {
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
	out, err := d.Command(versionCmd[0], versionCmd[1:]...).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to query FPMCU version")
	}
	re := regexp.MustCompile(`Firmware copy:\s+(RO|RW)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) != 2 {
		return "", errors.New("cannot find firmware copy string")
	}
	return matches[1], nil
}

// RollbackInfo returns the rollbackinfo of the fingerprint MCU.
func RollbackInfo(ctx context.Context, d *dut.DUT) ([]byte, error) {
	cmd := []string{"ectool", "--name=cros_fp", "rollbackinfo"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmd))
	out, err := d.Command(cmd[0], cmd[1:]...).Output(ctx)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to query FPMCU rollbackinfo")
	}
	return out, nil
}

// CheckRollbackSetToInitialValue checks the anti-rollback block is set to initial values.
func CheckRollbackSetToInitialValue(ctx context.Context, d *dut.DUT) error {
	return CheckRollbackState(ctx, d, 1, 0, 0)
}

// CheckRollbackState checks that the anti-rollback block is set to expected values.
func CheckRollbackState(ctx context.Context, d *dut.DUT, blockID, minVersion, rwVersion int) error {
	rollbackInfo, err := RollbackInfo(ctx, d)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`Rollback block id:\s+`+strconv.Itoa(blockID)).Match(rollbackInfo) ||
		!regexp.MustCompile(`Rollback min version:\s+`+strconv.Itoa(minVersion)).Match(rollbackInfo) ||
		!regexp.MustCompile(`RW rollback version:\s+`+strconv.Itoa(rwVersion)).Match(rollbackInfo) {
		testing.ContextLogf(ctx, "Rollback info: %q", string(rollbackInfo))
		return errors.New("Rollback not set to initial value")
	}
	return nil
}

// AddEntropy adds entropy to the fingerprint MCU.
func AddEntropy(ctx context.Context, d *dut.DUT, reset bool) error {
	cmd := []string{"ectool", "--name=cros_fp", "addentropy"}
	if reset {
		cmd = append(cmd, "reset")
	}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmd))
	return d.Command(cmd[0], cmd[1:]...).Run(ctx)
}

// parseColonDelimitedOutput parses colon delimited information to a map.
func parseColonDelimitedOutput(output string) map[string]string {
	ret := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		splits := strings.Split(line, ":")
		if len(splits) != 2 {
			continue
		}
		ret[strings.TrimSpace(splits[0])] = strings.TrimSpace(splits[1])
	}
	return ret
}
