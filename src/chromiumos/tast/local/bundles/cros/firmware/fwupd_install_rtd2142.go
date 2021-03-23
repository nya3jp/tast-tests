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

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInstallRtd2142,
		Desc: "Checks that fwupd can detect device and update the firmware",
		Contacts: []string{
			"sshiyu@chromium.org",          // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Platform("puff")),
		Data:         []string{"fwupd_install_fake_rtd2142_V99.99.cab", "flashrom"},
	})
}

// verifyFlashromCmdMst verifys the log file contains proper flashrom command.
func verifyFlashromCmdMst(path string) error {
	// flashrom is invoked three times, using a particular I2C bus:
	//  * Query flash size to infer layout
	//  * Write new firmware image
	//  * Write new boot flag
	const flashromCmdRegMst = `^-p realtek_mst_i2c_spi:bus=8,reset-mcu=1,enter-isp=1` +
		` --flash-size[\n]+` +
		`-p realtek_mst_i2c_spi:bus=8,reset-mcu=0,enter-isp=1` +
		` --layout /tmp/flashrom-i2c-8-[a-zA-Z0-9]{6}/layout` +
		` --image PARTITION[0-9]:/tmp/flashrom-i2c-8-[a-zA-Z0-9]{6}/rtd2142-V99.99.bin` +
		` -w[\n]+` +
		`-p realtek_mst_i2c_spi:bus=8,reset-mcu=1,enter-isp=0` +
		` --layout /tmp/flashrom-i2c-8-[a-zA-Z0-9]{6}/layout` +
		` --image FLAG[0-9]:/tmp/flashrom-i2c-8-[a-zA-Z0-9]{6}/flag[0-9].bin` +
		` -w[\n]*$`

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	matched, err := regexp.Match(flashromCmdRegMst, data)
	if err != nil {
		return err
	}

	if !matched {
		return errors.Errorf("failed to verify flashrom parameters: %q", data)
	}

	return nil
}

// FwupdInstallRtd2142 runs the fwupdtool utility update and verifies that the RTD2142
// device is recoginzed and gets updated as expected.
func FwupdInstallRtd2142(ctx context.Context, s *testing.State) {
	// Unique device ID hash for RTD2142 device used in fwupd.
	const deviceID = "778618d36946987157760aa552a44bdd0f39db12"
	flashromCallParam := filepath.Join(s.OutDir(), "fwupd_flashrom_call_parameters.txt")

	f, err := os.Create(filepath.Join(s.OutDir(), "fwupdtool-mst.txt"))
	if err != nil {
		s.Error("Failed to create fwupdtool output: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdtool", "install", s.DataPath("fwupd_install_fake_rtd2142_V99.99.cab"), deviceID)
	defer os.Remove(flashromCallParam)

	cmd.Stdout = f
	cmd.Env = []string{fmt.Sprintf("PATH=%s", filepath.Dir(s.DataPath("flashrom"))), fmt.Sprintf("FWUPD_TAST_OUT=%s", s.OutDir())}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyFlashromCmdMst(flashromCallParam); err != nil {
		s.Fatal("flashrom command failed to verify: ", err)
	}
}
