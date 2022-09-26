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

// captureDmesg saves the current dmesg to dmesg.txt so that we could debug
// possible cros-ec binding issues.
//
// If running the test manually, check the following file path:
// /tmp/tast/results/latest/tests/biod.CrosConfigMatchesDevice/dmesg.txt
func captureDmesg(ctx context.Context, s *testing.State) {
	s.Log("Please check the captured dmesg.txt for cros-ec binding issues")
	dmesg, err := testexec.CommandContext(ctx, "dmesg").CombinedOutput()
	if err != nil {
		s.Error("Failed to invoke dmesg to get possible bind error: ", err)
	}
	err = os.WriteFile(filepath.Join(s.OutDir(), "dmesg.txt"), dmesg, 0644)
	if err != nil {
		s.Error("Failed to write dmesg.txt: ", err)
	}
}

// CrosConfigMatchesDevice checks that cros-config fingerprint support
// matches the fingerprint device seen.
//
// 1. If cros-config supports fingerprint --> Expect /dev/cros_fp to exist
// 2. If /dev/cros_fp to exists --> Expect cros-config to support fingerprint
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
		s.Error("Cros-config supports fingerprint, but /dev/cros_fp is not preset")
		// We capture dmesg after to ensure that the above error is shown first.
		captureDmesg(ctx, s)
		return
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
