// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	flashromCmdRegLspcon = "^-p lspcon_i2c_spi:bus=7 --layout /tmp/flashrom-i2c-7-[a-zA-Z0-9]{6}/layout" +
		" --image PARTITION[0-9]:/tmp/flashrom-i2c-7-[a-zA-Z0-9]{6}/ps175-V99.99.bin -w[\n]+-p" +
		" lspcon_i2c_spi:bus=7 --layout /tmp/flashrom-i2c-7-[a-zA-Z0-9]{6}/layout --image" +
		" FLAG:/tmp/flashrom-i2c-7-[a-zA-Z0-9]{6}/flag[0-9].bin -w[\n]*$"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallPS175,
		Desc: "Checks that fwupd can detect device and update the firmware",
		Contacts: []string{
			"sshiyu@chromium.org",          // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Platform("puff")),
		Data:         []string{"fwupd_install_fake_ps175_V99.99.cab", "flashrom"},
	})
}

// verifyFlashromCmdLspcon verifys the log file contains proper flashrom command.
func verifyFlashromCmdLspcon(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	matched, err := regexp.Match(flashromCmdRegLspcon, data)
	if err != nil {
		return err
	}

	if !matched {
		return errors.Errorf("failed to verify flashrom parameters: %q", data)
	}

	return nil
}

// FwupdInstallPS175 runs the fwupdtool utility update and verifies that the PS175
// device is recoginzed and gets updated as expected.
func FwupdInstallPS175(ctx context.Context, s *testing.State) {
	const deviceID = "ef43eb9fd629d16aa4a1b86c30b9752a995f2a54"

	f, err := os.Create(filepath.Join(s.OutDir(), "fwupdtool-lspcon.txt"))
	if err != nil {
		s.Error("Failed to create fwupdtool output: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "install", s.DataPath("fwupd_install_fake_ps175_V99.99.cab"), deviceID)
	defer os.Remove(filepath.Join(s.OutDir(), "fwupd_flashrom_call_parameters.txt"))

	cmd.Stdout = f
	cmd.Env = []string{fmt.Sprintf("PATH=%s", filepath.Dir(s.DataPath("flashrom"))), fmt.Sprintf("FWUPD_TAST_OUT=%s", s.OutDir())}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyFlashromCmdLspcon(filepath.Join(s.OutDir(), "fwupd_flashrom_call_parameters.txt")); err != nil {
		s.Fatal("flashrom command failed to verify: ", err)
	}
}
