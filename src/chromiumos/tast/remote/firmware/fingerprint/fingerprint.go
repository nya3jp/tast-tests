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
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/upstart"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// RollbackState is the state of the anti-rollback block.
type RollbackState struct {
	BlockID    int
	MinVersion int
	RWVersion  int
}

// FWImageType is the type of firmware (RO or RW).
type FWImageType string

// These are the possible values of FWImageType.
const (
	ImageTypeRO FWImageType = "RO"
	ImageTypeRW FWImageType = "RW"
)

type firmwareMetadata struct {
	sha256sum string
	roVersion string
	rwVersion string
	keyID     string
}

type fwInfoType int

const (
	// Elements of firmware information.
	fwInfoTypeSha256sum fwInfoType = iota
	fwInfoTypeRoVersion
	fwInfoTypeRwVersion
	fwInfoTypeKeyID
)

type keyType string

const (
	// Types of signing keys.
	keyTypeDev   keyType = "dev"
	keyTypePreMp keyType = "premp"
	keyTypeMp    keyType = "mp"
)

// FPBoardName is the board name of the FPMCU.
type FPBoardName string

// Possible names for FPMCUs.
const (
	FPBoardNameBloonchipper FPBoardName = "bloonchipper"
	FPBoardNameDartmonkey   FPBoardName = "dartmonkey"
	FPBoardNameNocturne     FPBoardName = "nocturne_fp"
	FPBoardNameNami         FPBoardName = "nami_fp"
)

const (
	// nocturne and nami are special cases and have "_fp" appended.
	// Newer FPMCUs have unique names.
	// See go/cros-fingerprint-firmware-branching-and-signing.
	fingerprintBoardNameSuffix  = "_fp"
	fingerprintFirmwarePathBase = "/opt/google/biod/fw/"
	// WaitForBiodToStartTimeout is the time to wait for biod to start.
	WaitForBiodToStartTimeout = 30 * time.Second
	// TimeForCleanup is the amount of time to reserve for cleaning up firmware tests.
	TimeForCleanup       = 2 * time.Minute
	biodUpstartJobName   = "biod"
	powerdUpstartJobName = "powerd"
)

// Map from signing key ID to type of signing key.
var keyIDMap = map[string]keyType{
	// bloonchipper.
	"61382804da86b4156d666cc9a976088f8b647d44": keyTypeDev,
	"07b1af57220c196e363e68d73a5966047c77011e": keyTypePreMp,
	"1c590ef36399f6a2b2ef87079c135b69ef89eb60": keyTypeMp,

	// dartmonkey.
	"257a0aa3ac9e81aa4bc3aabdb6d3d079117c5799": keyTypeMp,

	// nocturne.
	"8a8fc039a9463271995392f079b83ce33832d07d": keyTypeDev,
	"6f38c866182bd9bf7a4462c06ac04fa6a0074351": keyTypeMp,
	"f6f7d96c48bd154dbae7e3fe3a3b4c6268a10934": keyTypePreMp,

	// nami.
	"754aea623d69975a22998f7b97315dd53115d723": keyTypePreMp,
	"35486c0090ca390408f1fbbf2a182966084fe2f8": keyTypeMp,
}

// Map of attributes for a given board's various firmware file releases.
// Two purposes:
//   1) Documents the exact versions and keys used for a given firmware file.
//   2) Used to verify that files that end up in the build (and therefore
//      what we release) is exactly what we expect.
var firmwareVersionMap = map[FPBoardName]map[string]firmwareMetadata{
	FPBoardNameBloonchipper: map[string]firmwareMetadata{
		"bloonchipper_v2.0.4277-9f652bb3-RO_v2.0.7314-3dfc5ff6-RW.bin": firmwareMetadata{
			sha256sum: "2bac89c16ad71986fe37ed651fe7dd6d5a3d039678d4a5f1d03c5a65a9f3bc3c",
			roVersion: "bloonchipper_v2.0.4277-9f652bb3",
			rwVersion: "bloonchipper_v2.0.7314-3dfc5ff6",
			keyID:     "1c590ef36399f6a2b2ef87079c135b69ef89eb60",
		},
		"bloonchipper_v2.0.5938-197506c1-RO_v2.0.7314-3dfc5ff6-RW.bin": firmwareMetadata{
			sha256sum: "50ddcad558e1ded476a209946cabcddd6d9c1033890f1661d7ba8c183aa625ab",
			roVersion: "bloonchipper_v2.0.5938-197506c1",
			rwVersion: "bloonchipper_v2.0.7314-3dfc5ff6",
			keyID:     "1c590ef36399f6a2b2ef87079c135b69ef89eb60",
		},
	},
	FPBoardNameNocturne: map[string]firmwareMetadata{
		"nocturne_fp_v2.2.64-58cf5974e-RO_v2.0.7304-441100b93-RW.bin": firmwareMetadata{
			sha256sum: "569a191bd2ed25ce89b296f0ab8cd2ed567dbf6a8df3f6b3f82ad58c786d79a9",
			roVersion: "nocturne_fp_v2.2.64-58cf5974e",
			rwVersion: "nocturne_fp_v2.0.7304-441100b93",
			keyID:     "6f38c866182bd9bf7a4462c06ac04fa6a0074351",
		},
	},
	FPBoardNameNami: map[string]firmwareMetadata{
		"nami_fp_v2.2.144-7a08e07eb-RO_v2.0.7304-441100b93-RW.bin": firmwareMetadata{
			sha256sum: "e7b23f5e585c47d24fe3696139b48c0bac8c43b025669f74aafbff4aa9cbbebd",
			roVersion: "nami_fp_v2.2.144-7a08e07eb",
			rwVersion: "nami_fp_v2.0.7304-441100b93",
			keyID:     "35486c0090ca390408f1fbbf2a182966084fe2f8",
		},
	},
	FPBoardNameDartmonkey: map[string]firmwareMetadata{
		"dartmonkey_v2.0.2887-311310808-RO_v2.0.7304-441100b93-RW.bin": firmwareMetadata{
			sha256sum: "5127137655b4b13d7a86ba897b08a9957d36b74afb97558496c6fba98e808b7b",
			roVersion: "dartmonkey_v2.0.2887-311310808",
			rwVersion: "dartmonkey_v2.0.7304-441100b93",
			keyID:     "257a0aa3ac9e81aa4bc3aabdb6d3d079117c5799",
		},
	},
}

type daemonState struct {
	name       string
	wasRunning bool // true if daemon was originally running
}

// FirmwareTest provides a common framework for fingerprint firmware tests.
type FirmwareTest struct {
	d              *dut.DUT
	servo          *servo.Proxy
	cl             *rpc.Client
	rpcHint        *testing.RPCHint
	fpBoard        FPBoardName
	buildFwFile    string
	upstartService platform.UpstartServiceClient
	daemonState    []daemonState
}

// NewFirmwareTest creates and initializes a new fingerprint firmware test.
// enableHWWP indicates whether the test should enable hardware write protect.
// enableSWWP indicates whether the test should enable software write protect.
func NewFirmwareTest(ctx context.Context, d *dut.DUT, servoSpec string, hint *testing.RPCHint, outDir string, enableHWWP, enableSWWP bool) (*FirmwareTest, error) {
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to servo")
	}

	cl, err := rpc.Dial(ctx, d, hint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}

	upstartService := platform.NewUpstartServiceClient(cl.Conn)

	daemonState, err := stopDaemons(ctx, upstartService, []string{
		biodUpstartJobName,
		// TODO(b/183123775): Remove when bug is fixed.
		//  Disabling powerd to prevent the display from turning off, which kills
		//  USB on some platforms.
		powerdUpstartJobName,
	})
	if err != nil {
		return nil, err
	}

	fpBoard, err := Board(ctx, d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get fingerprint board")
	}

	buildFwFile, err := FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get build firmware file path")
	}

	if err := ValidateBuildFwFile(ctx, d, fpBoard, buildFwFile); err != nil {
		return nil, errors.Wrap(err, "failed to validate build firmware file")
	}

	if err := InitializeKnownState(ctx, d, outDir, pxy); err != nil {
		return nil, errors.Wrap(err, "initializing known state failed")
	}

	// TODO(b/182596510): Check the FPMCU is running expected firmware version.

	if err := InitializeHWAndSWWriteProtect(ctx, d, pxy, enableHWWP, enableSWWP); err != nil {
		return nil, errors.Wrap(err, "initializing write protect failed")
	}

	return &FirmwareTest{
			d:              d,
			servo:          pxy,
			cl:             cl,
			rpcHint:        hint,
			fpBoard:        fpBoard,
			buildFwFile:    buildFwFile,
			upstartService: upstartService,
			daemonState:    daemonState,
		},
		nil
}

// Close cleans up the fingerprint test and restore the FPMCU firmware to the
// original image and state.
func (t *FirmwareTest) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down")
	var firstErr error

	if err := ReimageFPMCU(ctx, t.d, t.servo); err != nil {
		firstErr = err
	}

	// TODO(https://crbug.com/1195936): ReimageFPMCU reboots, which causes gRPC
	//  to lose its connection.
	cl, err := rpc.Dial(ctx, t.d, t.rpcHint, "cros")
	if err != nil && firstErr == nil {
		firstErr = err
	}

	if cl != nil {
		t.cl = cl
		t.upstartService = platform.NewUpstartServiceClient(cl.Conn)

		if err := restoreDaemons(ctx, t.upstartService, t.daemonState); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	t.servo.Close(ctx)

	if err := t.cl.Close(ctx); err != nil && firstErr == nil {
		// TODO(https://crbug.com/1196734): this always fails with "context canceled",
		//  so disabling for now
		// firstErr = err
	}

	return firstErr
}

// DUT gets the DUT.
func (t *FirmwareTest) DUT() *dut.DUT {
	return t.d
}

// Servo gets the servo proxy.
func (t *FirmwareTest) Servo() *servo.Proxy {
	return t.servo
}

// RPCClient gets the RPC client.
func (t *FirmwareTest) RPCClient() *rpc.Client {
	return t.cl
}

// BuildFwFile gets the firmware file.
func (t *FirmwareTest) BuildFwFile() string {
	return t.buildFwFile
}

// stopDaemons stops the specified daemons and returns their original state.
func stopDaemons(ctx context.Context, upstartService platform.UpstartServiceClient, daemons []string) ([]daemonState, error) {
	var ret []daemonState
	for _, name := range daemons {
		status, err := upstartService.JobStatus(ctx, &platform.JobStatusRequest{JobName: name})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get status for"+name)
		}

		daemonWasRunning := upstart.State(status.GetState()) == upstart.RunningState

		if daemonWasRunning {
			testing.ContextLog(ctx, "Stopping ", name)
			_, err := upstartService.StopJob(ctx, &platform.StopJobRequest{
				JobName: name,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to stop "+name)
			}
		}

		ret = append(ret, daemonState{
			name:       name,
			wasRunning: daemonWasRunning,
		})
	}

	return ret, nil
}

// restoreDaemons restores the daemons to the state provided in daemonState.
func restoreDaemons(ctx context.Context, upstartService platform.UpstartServiceClient, daemons []daemonState) error {
	var firstErr error

	for i := len(daemons) - 1; i >= 0; i-- {
		daemon := daemons[i]

		testing.ContextLog(ctx, "Checking state for ", daemon.name)
		status, err := upstartService.JobStatus(ctx, &platform.JobStatusRequest{JobName: daemon.name})
		if err != nil {
			testing.ContextLog(ctx, "Failed to get state for "+daemon.name)
			if firstErr != nil {
				firstErr = err
			}
			continue
		}

		running := upstart.State(status.GetState()) == upstart.RunningState

		if running != daemon.wasRunning {
			if running {
				testing.ContextLog(ctx, "Stopping ", daemon.name)
				_, err := upstartService.StopJob(ctx, &platform.StopJobRequest{
					JobName: daemon.name,
				})
				if err != nil {
					testing.ContextLog(ctx, "Failed to stop "+daemon.name)
					if firstErr != nil {
						firstErr = err
					}
				}
			} else {
				testing.ContextLog(ctx, "Starting ", daemon.name)
				_, err := upstartService.StartJob(ctx, &platform.StartJobRequest{
					JobName: daemon.name,
				})
				if err != nil {
					testing.ContextLog(ctx, "Failed to start "+daemon.name)
					if firstErr != nil {
						firstErr = err
					}
				}
			}
		}
	}
	return firstErr
}

// getExpectedFwInfo returns expected firmware info for a given firmware file name.
func getExpectedFwInfo(fpBoard FPBoardName, buildFwFile string, infoType fwInfoType) (string, error) {
	boardExpectedFwInfo, ok := firmwareVersionMap[fpBoard]
	if !ok {
		return "", errors.Errorf("failed to get firmware info for board %s", fpBoard)
	}
	expectedFwInfo, ok := boardExpectedFwInfo[filepath.Base(buildFwFile)]
	if !ok {
		return "", errors.Errorf("failed to get firmware info for file %s", buildFwFile)
	}
	switch infoType {
	case fwInfoTypeSha256sum:
		return expectedFwInfo.sha256sum, nil
	case fwInfoTypeRwVersion:
		return expectedFwInfo.rwVersion, nil
	case fwInfoTypeRoVersion:
		return expectedFwInfo.roVersion, nil
	case fwInfoTypeKeyID:
		return expectedFwInfo.keyID, nil
	default:
		return "", errors.Errorf("failed to get firmware info type %d", infoType)
	}
}

// ValidateBuildFwFile checks that all attributes in the given firmware file match their expected values.
func ValidateBuildFwFile(ctx context.Context, d *dut.DUT, fpBoard FPBoardName, buildFwFile string) error {
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
		return errors.Errorf("failed to validate the sha256 sum, got %s, want %s", actualHash, expectedHash)
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
		return errors.Errorf("failed to validate the key id, got %s, want %s", actualKeyID, expectedKeyID)
	}

	// Check the signing key type is allowed.
	keyType, ok := keyIDMap[actualKeyID]
	if !ok {
		return errors.Errorf("failed to get key type for key id: %s", actualKeyID)
	}
	if keyType != keyTypePreMp && keyType != keyTypeMp {
		return errors.Errorf("key type %s is not allowed", keyType)
	}

	// Check RO version.
	actualRoVersion, err := readFmapSection(ctx, d, buildFwFile, "RO_FRID")
	if err != nil {
		return err
	}
	expectedRoVersion, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeRoVersion)
	if err != nil {
		return err
	}
	if actualRoVersion != expectedRoVersion {
		return errors.Errorf("failed to validate the RO version, got %s, want %s", actualRoVersion, expectedRoVersion)
	}

	// Check RW version.
	actualRwVersion, err := GetBuildRWFirmwareVersion(ctx, d, buildFwFile)
	if err != nil {
		return err
	}
	expectedRwVersion, err := getExpectedFwInfo(fpBoard, buildFwFile, fwInfoTypeRwVersion)
	if err != nil {
		return err
	}
	if actualRwVersion != expectedRwVersion {
		return errors.Errorf("failed to validate the RW version, got %s, want %s", actualRwVersion, expectedRwVersion)
	}

	testing.ContextLog(ctx, "Succeeded validating build firmware metadata")
	return nil
}

// GetBuildRWFirmwareVersion returns the RW version of a given build firmware file on DUT.
func GetBuildRWFirmwareVersion(ctx context.Context, d *dut.DUT, buildFWFile string) (string, error) {
	return readFmapSection(ctx, d, buildFWFile, "RW_FWID")
}

// readFmapSection reads a section (e.g. RO_FRID) from a firmware file on device.
func readFmapSection(ctx context.Context, d *dut.DUT, buildFwFile, section string) (string, error) {
	// TODO(crbug:1189908): Call library once it's available.
	// Prepare a temporary file because dump_map only writes the
	// value read from a section to a file (will not just print it to
	// stdout).
	tempdir, err := d.Conn().Command("mktemp", "-d", "/tmp/fingerprint_dump_fmap_XXXXXX").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to create remote temp directory")
	}
	tempdirPath := strings.TrimSpace(string(tempdir))
	defer d.Conn().Command("rm", "-r", tempdirPath).Run(ctx, ssh.DumpLogOnError)

	outputPath := filepath.Join(tempdirPath, section)
	if err := d.Conn().Command("dump_fmap", "-x", buildFwFile, fmt.Sprintf("%s:%s", section, outputPath)).Run(ctx, ssh.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to run dump_fmap")
	}

	out, err := d.Conn().Command("cat", outputPath).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to read dump_fmap output")
	}
	// dump_fmap writes NULL characters at the end.
	return strings.Trim(string(out), "\x00"), nil
}

// readFirmwareKeyID reads the key id of a firmware file on device.
func readFirmwareKeyID(ctx context.Context, d *dut.DUT, buildFwFile string) (string, error) {
	out, err := d.Conn().Command("futility", "show", buildFwFile).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run futility on device")
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
	out, err := d.Conn().Command("sha256sum", buildFwFile).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to calculate sha256sum on device")
	}
	return strings.Split(string(out), " ")[0], nil
}

// boardFromCrosConfig returns the fingerprint board name from cros_config.
func boardFromCrosConfig(ctx context.Context, d *dut.DUT) (FPBoardName, error) {
	out, err := d.Conn().Command("cros_config", "/fingerprint", "board").Output(ctx, ssh.DumpLogOnError)
	return FPBoardName(out), err
}

// Board returns the name of the fingerprint EC on the DUT
func Board(ctx context.Context, d *dut.DUT) (FPBoardName, error) {
	// For devices that don't have unibuild support (which is required to
	// use cros_config).
	// TODO(https://crbug.com/1030862): remove when nocturne has cros_config
	// support.
	board, err := reporters.New(d).Board(ctx)
	if err != nil {
		return FPBoardName(""), err
	}
	if board == "nocturne" {
		return FPBoardName(board + fingerprintBoardNameSuffix), nil
	}

	// Use cros_config to get fingerprint board.
	return boardFromCrosConfig(ctx, d)
}

// FirmwarePath returns the path to the fingerprint firmware file on device.
func FirmwarePath(ctx context.Context, d *dut.DUT, fpBoard FPBoardName) (string, error) {
	cmd := fmt.Sprintf("ls %s%s*.bin", fingerprintFirmwarePathBase, fpBoard)
	out, err := d.Conn().Command("bash", "-c", cmd).Output(ctx, ssh.DumpLogOnError)
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
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(flashCmd))
	if err := d.Conn().Command(flashCmd[0], flashCmd[1:]...).Run(ctx, ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	return nil
}

// InitializeEntropy initializes the anti-rollback block in RO firmware.
func InitializeEntropy(ctx context.Context, d *dut.DUT) error {
	if err := d.Conn().Command("bio_wash", "--factory_init").Run(ctx, ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to initialize entropy")
	}
	return nil
}

// CheckFirmwareIsFunctional checks that the AP can talk to the FPMCU and get the version.
func CheckFirmwareIsFunctional(ctx context.Context, d *dut.DUT) ([]byte, error) {
	testing.ContextLog(ctx, "Checking firmware is functional")
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(versionCmd))
	return d.Conn().Command(versionCmd[0], versionCmd[1:]...).Output(ctx, ssh.DumpLogOnError)
}

// ReimageFPMCU flashes the FPMCU completely and initializes entropy.
func ReimageFPMCU(ctx context.Context, d *dut.DUT, pxy *servo.Proxy) error {
	if err := pxy.Servo().SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
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
	if err := pxy.Servo().SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		return errors.Wrap(err, "failed to enable HW write protect")
	}
	return nil
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
		return ReimageFPMCU(ctx, d, pxy)
	}
	return nil
}

// InitializeHWAndSWWriteProtect ensures hardware and software write protect are initialized as requested.
func InitializeHWAndSWWriteProtect(ctx context.Context, d *dut.DUT, pxy *servo.Proxy, enableHWWP, enableSWWP bool) error {
	testing.ContextLogf(ctx, "Initializing HW WP to %t, SW WP to %t", enableHWWP, enableSWWP)
	// HW write protect must be disabled to disable SW write protect.
	hwWPArg := servo.FWWPStateOff
	if !enableSWWP {
		if err := pxy.Servo().SetFWWPState(ctx, hwWPArg); err != nil {
			return errors.Wrap(err, "failed to disable HW write protect")
		}
	}

	swWPArg := "disable"
	if enableSWWP {
		swWPArg = "enable"
	}
	// TODO(b/116396469): Add error checking once it's fixed.
	// This command can return error even on success, so ignore error for now.
	_ = d.Conn().Command("ectool", "--name=cros_fp", "flashprotect", swWPArg).Run(ctx)
	// TODO(b/116396469): "flashprotect enable" command is slow, so wait for
	// it to complete before attempting to reboot.
	testing.Sleep(ctx, 2*time.Second)
	if err := RebootFpmcu(ctx, d, "RW"); err != nil {
		return err
	}

	if enableHWWP {
		hwWPArg = servo.FWWPStateOn
	}
	if err := pxy.Servo().SetFWWPState(ctx, hwWPArg); err != nil {
		return errors.Wrapf(err, "failed to set HW write protect to %q", hwWPArg)
	}
	// TODO(b/182597335): Check the correct flags, which is different for different chips.
	return nil
}

// RebootFpmcu reboots the fingerprint MCU. It does not reboot the AP.
func RebootFpmcu(ctx context.Context, d *dut.DUT, bootTo FWImageType) error {
	testing.ContextLog(ctx, "Rebooting FPMCU")
	// This command returns error even on success, so ignore error. b/116396469
	_ = d.Conn().Command("ectool", "--name=cros_fp", "reboot_ec").Run(ctx)
	if bootTo == ImageTypeRO {
		testing.Sleep(ctx, 500*time.Millisecond)
		err := d.Conn().Command("ectool", "--name=cros_fp", "rwsigaction", "abort").Run(ctx)
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

// RunningFirmwareCopy returns the firmware copy on FPMCU (RO or RW).
func RunningFirmwareCopy(ctx context.Context, d *dut.DUT) (FWImageType, error) {
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(versionCmd))
	out, err := d.Conn().Command(versionCmd[0], versionCmd[1:]...).Output(ctx)
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

// RunningRWVersion returns the RW version running on FPMCU.
func RunningRWVersion(ctx context.Context, d *dut.DUT) (string, error) {
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(versionCmd))
	out, err := d.Conn().Command(versionCmd[0], versionCmd[1:]...).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to query FPMCU version")
	}
	versionInfoMap := parseColonDelimitedOutput(string(out))
	return versionInfoMap["RW version"], nil
}

// RollbackInfo returns the rollbackinfo of the fingerprint MCU.
func RollbackInfo(ctx context.Context, d *dut.DUT) ([]byte, error) {
	cmd := []string{"ectool", "--name=cros_fp", "rollbackinfo"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd))
	out, err := d.Conn().Command(cmd[0], cmd[1:]...).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to query FPMCU rollbackinfo")
	}
	return out, nil
}

// CheckRollbackSetToInitialValue checks the anti-rollback block is set to initial values.
func CheckRollbackSetToInitialValue(ctx context.Context, d *dut.DUT) error {
	return CheckRollbackState(ctx, d, RollbackState{
		BlockID:    1,
		MinVersion: 0,
		RWVersion:  0,
	})
}

// CheckRollbackState checks that the anti-rollback block is set to expected values.
func CheckRollbackState(ctx context.Context, d *dut.DUT, expected RollbackState) error {
	rollbackInfo, err := RollbackInfo(ctx, d)
	if err != nil {
		return err
	}
	rollbackInfoMap := parseColonDelimitedOutput(string(rollbackInfo))

	var actual RollbackState
	blockID, err := strconv.Atoi(rollbackInfoMap["Rollback block id"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback block id")
	}
	actual.BlockID = blockID

	minVersion, err := strconv.Atoi(rollbackInfoMap["Rollback min version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback min version")
	}
	actual.MinVersion = minVersion

	rwVersion, err := strconv.Atoi(rollbackInfoMap["RW rollback version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert RW rollback version")
	}
	actual.RWVersion = rwVersion

	if actual != expected {
		return errors.Errorf("Rollback not set correctly, expected: %q, actual: %q", expected, actual)
	}

	return nil
}

// AddEntropy adds entropy to the fingerprint MCU.
func AddEntropy(ctx context.Context, d *dut.DUT, reset bool) error {
	cmd := []string{"ectool", "--name=cros_fp", "addentropy"}
	if reset {
		cmd = append(cmd, "reset")
	}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd))
	return d.Conn().Command(cmd[0], cmd[1:]...).Run(ctx)
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
