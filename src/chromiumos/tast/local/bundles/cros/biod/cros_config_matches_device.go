// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	fp "chromiumos/tast/common/fingerprint"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfigMatchesDevice,
		Desc: "Checks that fingerprint support in cros-config agrees with the device/driver",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// Note that hwdep for fingerprint relies on cros-config.
		// We omit any dependencies on hwdep fingerprint so that we can detect
		// issues where cros-config fails to mention fingerprint support.
	})
}

// CrosConfigMatchesDevice checks that cros-config fingerprint support
// matches the fingerprint device seen.
//
// 1. If cros-config supports fingerprint --> Expect /dev/cros_fp to exist
// 2. If /dev/cros_fp exists --> Expect cros-config to support fingerprint
// 3. Ensure that cros-config board matches the actual FPMCU version/board.
func CrosConfigMatchesDevice(ctx context.Context, s *testing.State) {
	sensorLocation, err := crosconfig.Get(ctx, "/fingerprint", "sensor-location")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to invoke cros_config for sensor-location: ", err)
	}
	cfgFpSupported := fp.SensorLoc(sensorLocation).IsSupported()

	// Reading from /dev/cros_fp tells us if the file exists and provides the
	// FPMCU version.
	version, err := os.ReadFile("/dev/cros_fp")
	if err != nil && !os.IsNotExist(err) {
		s.Fatal("Failed to read /dev/cros_fp for reasons other than does-not-exist: ", err)
	}
	fpDevNotExists := os.IsNotExist(err)
	fpDevExists := err == nil

	if cfgFpSupported && fpDevNotExists {
		defer func(ctx context.Context) {
			if err := captureDmesg(ctx, s.OutDir()); err != nil {
				s.Fatal("Failed to capture dmesg: ", err)
			}
		}(ctx)
		s.Fatal("Cros-config supports fingerprint, but /dev/cros_fp is not preset")
	}
	if fpDevExists && !cfgFpSupported {
		s.Fatal("The /dev/cros_fp device exists, but cros-config doesn't support fingerprint")
	}

	if !cfgFpSupported {
		// Fingerprint appears to be properly unsupported on device.
		return
	}

	board, err := crosconfig.Get(ctx, "/fingerprint", "board")
	if err != nil {
		s.Fatal("Failed to get board from cros_config: ", err)
	}
	if ver := string(version); !strings.Contains(ver, board) {
		s.Fatalf("The cros-config board %q doesn't match FPMCU version %q", board, ver)
	}
}

// captureDmesg saves the current dmesg to dmesg.txt so that we could debug
// possible cros-ec binding issues.
//
// Please check the captured dmesg.txt for cros-ec binding issues.
// The output file can be found using go/tast-running#interpreting-test-results.
func captureDmesg(ctx context.Context, dir string) error {
	dmesg, err := testexec.CommandContext(ctx, "dmesg").CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "failed to invoke dmesg to get possible bind error")
	}
	if err := os.WriteFile(filepath.Join(dir, "dmesg.txt"), dmesg, 0644); err != nil {
		return errors.Wrap(err, "failed to write dmesg.txt")
	}
	return nil
}
