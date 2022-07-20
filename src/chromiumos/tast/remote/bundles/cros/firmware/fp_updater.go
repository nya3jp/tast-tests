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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/services/cros/firmware"
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
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.firmware.FpUpdaterService", "tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
		Data: []string{"nocturne_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"nami_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"bloonchipper_v2.0.14206-ad46faf_20220718.bin",
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
		return s.DataPath("bloonchipper_v2.0.14206-ad46faf_20220718.bin"), nil
	case fingerprint.FPBoardNameDartmonkey:
		return s.DataPath("dartmonkey_v2.0.2887-311310808_20201214.bin"), nil
	default:
		return "", errors.Errorf("no old firmware for %q", fpBoard)
	}
}

// flashOldRWFirmware flashes a known out-dated version of firmware.
func flashOldRWFirmware(ctx context.Context, s *testing.State, d *rpcdut.RPCDUT) error {
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

	if err := fingerprint.FlashRWFirmware(ctx, d, oldFirmwarePathOnDut); err != nil {
		return errors.Wrap(err, "failed to flash RW firmware")
	}

	// Check if FPMCU is running flashed RW.
	buildRWVersion, err := fingerprint.GetBuildRWFirmwareVersion(ctx, d, oldFirmwarePathOnDut)
	if err != nil {
		return errors.Wrap(err, "failed to query build RW version")
	}

	runningRWVersion, err := fingerprint.RunningRWVersion(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to query running RW version")
	}

	testing.ContextLogf(ctx, "Running RW version: %q", runningRWVersion)
	if runningRWVersion != buildRWVersion {
		return errors.Errorf("FPMCU is running incorrect RW version. Running %s, want %s", runningRWVersion, buildRWVersion)
	}

	return nil
}

func FpUpdater(ctx context.Context, s *testing.State) {
	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer d.Close(ctx)

	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}
	// Set SW write protect to true to enable RDP1 and HW write protect to true.
	t, err := fingerprint.NewFirmwareTest(ctx, d, servoSpec, s.OutDir(), true /*HW protect*/, true /*SW protect*/)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	cleanupCtx := ctx
	defer func() {
		if err := t.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	// If FP updater disabled, enable it.
	fpUpdaterEnabled, err := fingerprint.IsFPUpdaterEnabled(ctx, d)
	if err != nil {
		s.Fatal("Failed to check FP updater state: ", err)
	}
	if !fpUpdaterEnabled {
		if err := fingerprint.EnableFPUpdater(ctx, d); err != nil {
			s.Fatal("Failed to enable FP updater: ", err)
		}
	}

	testing.ContextLog(ctx, "Flashing outdated FP firmware")
	if err := flashOldRWFirmware(ctx, s, d); err != nil {
		s.Fatal("Failed to flash outdated RW firmware: ", err)
	}

	testing.ContextLog(ctx, "Rebooting DUT to invoke fp updater")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	fpUpdaterService := firmware.NewFpUpdaterServiceClient(d.RPC().Conn)
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

	// DUT reboots after FPMCU firmware update, so the previous file
	// contains log from update. The latest file should contain information
	// that update is not necessary.
	if !strings.Contains(previous, successString) {
		s.Fatal("Updater did not succeed, please check output dir")
	}

	buildRWVersion, err := fingerprint.GetBuildRWFirmwareVersion(ctx, d, t.BuildFwFile())
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
