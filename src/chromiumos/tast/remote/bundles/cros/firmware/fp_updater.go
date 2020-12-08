// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/shutil"
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
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
		Vars:         []string{"servo"},
		Data: []string{"nocturne_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"nami_fp_v2.0.3266-99b5e2c98_20201214.bin",
			"bloonchipper_v2.0.2972-e53c1977_20201214.bin",
			"dartmonkey_v2.0.2887-311310808_20201214.bin"},
	})
}

// getOldFirmwarePath returns the path to a known out-dated firmware.
func getOldFirmwarePath(s *testing.State, fpBoard string) (string, error) {
	switch fpBoard {
	case "nocturne_fp":
		return s.DataPath("nocturne_fp_v2.0.3266-99b5e2c98_20201214.bin"), nil
	case "nami_fp":
		return s.DataPath("nami_fp_v2.0.3266-99b5e2c98_20201214.bin"), nil
	case "bloonchipper":
		return s.DataPath("bloonchipper_v2.0.2972-e53c1977_20201214.bin"), nil
	case "dartmonkey":
		return s.DataPath("dartmonkey_v2.0.2887-311310808_20201214.bin"), nil
	default:
		return "", errors.Errorf("no old firmware for %q", fpBoard)
	}
}

// flashOldRWFirmware flashes a known out-dated version of firmware.
func flashOldRWFirmware(ctx context.Context, s *testing.State, d *dut.DUT) error {
	fpBoard, err := fingerprint.GetFpBoard(ctx, d)
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
	flashCmd := []string{"flashrom", "--fast-verify", "-w", oldFirmwarePathOnDut, "-i", "EC_RW", "-p", "ec:type=fp"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if err := d.Command(flashCmd[0], flashCmd[1:]...).Run(ctx); err != nil {
		return errors.Wrap(err, "flashrom failed")
	}
	return nil
}

// readUpdaterLogs reads the latest and previous updater logs as strings.
func readUpdaterLogs(ctx context.Context, s *testing.State, d *dut.DUT) (string, string) {
	latestData, err := d.Command("cat", latestLog).Output(ctx)
	if err != nil {
		s.Fatal("Failed to read latest updater log: ", err)
	}

	previousData, err := d.Command("cat", previousLog).Output(ctx)
	if err != nil {
		// Previous log doesn't exist, this is the first boot.
		previousData = []byte{}
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), path.Base(latestLog)), latestData, 0644); err != nil {
		s.Error("Failed to write latest updater log to file: ", err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), path.Base(previousLog)), previousData, 0644); err != nil {
		s.Error("Failed to write previous updater log to file: ", err)
	}

	return strings.TrimSpace(string(latestData)), strings.TrimSpace(string(previousData))
}

func FpUpdater(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if err := fingerprint.InitializeKnownState(ctx, d, s.OutDir()); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_on"); err != nil {
		s.Fatal("Failed to enable HW write protect: ", err)
	}

	defer cleanup(ctx, s, d, pxy)

	testing.ContextLog(ctx, "Flashing old FP firmware")
	if err := flashOldRWFirmware(ctx, s, d); err != nil {
		s.Fatal("Failed to flash old RW firmware: ", err)
	}

	testing.ContextLog(ctx, "Invoking fp updater")
	// The updater issues a reboot after update so don't expect response.
	if err := d.Command("bio_fw_updater").Start(ctx); err != nil {
		s.Fatal("Failed to execute bio_fw_updater: ", err)
	}
	s.Log("Waiting for update and reboot")
	testing.Sleep(ctx, 60*time.Second)

	s.Log("Reconnecting to DUT")
	if err := d.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}

	latest, _ := readUpdaterLogs(ctx, s, d)

	if !strings.Contains(latest, successString) {
		s.Fatal("Updater did not succeed, please check output dir")
	}
}

// cleanup restores the original fingerprint firmware.
func cleanup(ctx context.Context, s *testing.State, d *dut.DUT, pxy *servo.Proxy) {
	testing.ContextLog(ctx, "Starting cleanup at the end of test")
	if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_off"); err != nil {
		s.Fatal("Failed to disable HW write protect: ", err)
	}

	if err := fingerprint.FlashFpFirmware(ctx, d); err != nil {
		s.Error("Failed to flash original FP firmware: ", err)
	}
}
