// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	latestLog     = "/var/log/biod/bio_fw_updater.LATEST"
	previousLog   = "/var/log/biod/bio_fw_updater.PREVIOUS"
	successString = "The update was successful."
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpUpdater,
		Desc: "Checks that the fingerprint firmware updater succeeds when an update is needed",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.firmware.FpUpdaterService", dutfs.ServiceName},
		Vars:         []string{"servo"},
		Data: []string{"nocturne_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"nami_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"bloonchipper_v2.0.4277-9f652bb3_20210401.bin",
			"dartmonkey_v2.0.2887-311310808_20201214.bin"},
	})
}

// getOldFirmwarePath returns the path to a known out-dated firmware.
func getOldFirmwarePath(s *testing.State, fpBoard fingerprint.FPBoardName) (string, error) {
	switch fpBoard {
	case fingerprint.FPBoardNameNocturne:
		return s.DataPath("nocturne_fp_v2.0.3266-99b5e2c98_20201214.bin"), nil
	case fingerprint.FPBoardNameNami:
		return s.DataPath("nami_fp_v2.0.3266-99b5e2c98_20201214.bin"), nil
	case fingerprint.FPBoardNameBloonchipper:
		return s.DataPath("bloonchipper_v2.0.4277-9f652bb3_20210401.bin"), nil
	case fingerprint.FPBoardNameDartmonkey:
		return s.DataPath("dartmonkey_v2.0.2887-311310808_20201214.bin"), nil
	default:
		return "", errors.Errorf("no old firmware for %q", fpBoard)
	}
}

// flashOldRWFirmware flashes a known out-dated version of firmware.
func flashOldRWFirmware(ctx context.Context, s *testing.State, d *dut.DUT) error {
	fpBoard, err := fingerprint.Board(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get fp board")
	}
	testing.ContextLogf(ctx, "fp board name: %q", fpBoard)
	oldFirmwarePath, err := getOldFirmwarePath(s, fpBoard)
	if err != nil {
		return errors.Wrap(err, "failed to get old firmware path")
	}
	oldFirmwarePathOnDut := filepath.Join("/tmp", path.Base(oldFirmwarePath))
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(), map[string]string{oldFirmwarePath: oldFirmwarePathOnDut},
		linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed to send old firmware to DUT")
	}
	flashCmd := []string{"flashrom", "--noverify-all", "-w", oldFirmwarePathOnDut, "-i", "EC_RW", "-p", "ec:type=fp"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if err := d.Conn().Command(flashCmd[0], flashCmd[1:]...).Run(ctx, ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "flashrom failed")
	}
	return nil
}

func FpUpdater(ctx context.Context, s *testing.State) {
	d := s.DUT()

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	needsReboot, err := fingerprint.NeedsRebootAfterFlashing(ctx, d)
	if err != nil {
		s.Fatal("Failed to determine whether reboot is needed: ", err)
	}

	defer func() {
		if err := fingerprint.ReimageFPMCU(ctx, d, pxy, needsReboot); err != nil {
			s.Error("Failed to flash original firmware: ", err)
		}
	}()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	dutfsClient := dutfs.NewClient(cl.Conn)

	fpBoard, err := fingerprint.Board(ctx, d)
	if err != nil {
		s.Fatal("Failed to get fingerprint board: ", err)
	}

	buildFWFile, err := fingerprint.FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		s.Fatal("Failed to get build firmware file path: ", err)
	}

	// InitializeKnownState enables HW write protect so that we are testing
	// the same configuration as the end user.
	if err := fingerprint.InitializeKnownState(ctx, d, dutfsClient, s.OutDir(), pxy, fpBoard, buildFWFile, needsReboot); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	testing.ContextLog(ctx, "Flashing outdated FP firmware")
	if err := flashOldRWFirmware(ctx, s, d); err != nil {
		s.Fatal("Failed to flash outdated RW firmware: ", err)
	}

	testing.ContextLog(ctx, "Invoking fp updater")
	// The updater issues a reboot after update so don't expect response.
	if err := d.Conn().Command("bio_fw_updater").Start(ctx); err != nil {
		s.Fatal("Failed to execute bio_fw_updater: ", err)
	}
	s.Log("Waiting for update and reboot")
	testing.Sleep(ctx, 60*time.Second)

	s.Log("Reconnecting to DUT")
	if err := d.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	fpUpdaterService := firmware.NewFpUpdaterServiceClient(cl.Conn)
	response, err := fpUpdaterService.ReadUpdaterLogs(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to read updater logs: ", err)
	}
	latest := response.GetLatestLog()
	previous := response.GetPreviousLog()

	latestLogFile, err := os.Create(filepath.Join(s.OutDir(), path.Base(latestLog)))
	if err != nil {
		s.Error("Failed to write latest updater log to file: ", err)
	}
	defer latestLogFile.Close()
	if _, err := latestLogFile.WriteString(latest); err != nil {
		s.Error("Failed to write latest updater log to file: ", err)
	}
	previousLogFile, err := os.Create(filepath.Join(s.OutDir(), path.Base(previousLog)))
	if err != nil {
		s.Error("Failed to write previous updater log to file: ", err)
	}
	defer previousLogFile.Close()
	if _, err := previousLogFile.WriteString(previous); err != nil {
		s.Error("Failed to write previous updater log to file: ", err)
	}

	if !strings.Contains(latest, successString) {
		s.Fatal("Updater did not succeed, please check output dir")
	}

	buildRWVersion, err := fingerprint.GetBuildRWFirmwareVersion(ctx, d, dutfs.NewClient(cl.Conn), buildFWFile)
	if err != nil {
		s.Fatal("Failed to query build RW version: ", err)
	}

	runningRWVersion, err := fingerprint.RunningRWVersion(ctx, d)
	if err != nil {
		s.Fatal("Failed to query running RW version: ", err)
	}

	if runningRWVersion != buildRWVersion {
		s.Fatalf("Running RW version %s, want %s", runningRWVersion, buildRWVersion)
	}
}
