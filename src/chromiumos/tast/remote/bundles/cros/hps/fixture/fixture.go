// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/remote/sysutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// HpsdUsingLatestFirmware is a fixture which reconfigures hpsd to use
	// latest (i.e. ToT, unreleased) firmware from the ChromeOS test image,
	// rather than the released firmware it normally uses.
	HpsdUsingLatestFirmware = "hpsdUsingLatestFirmware"

	overrideFilename     = "/etc/init/hpsd.override"
	overrideOrigFilename = "/etc/init/hpsd.override.orig"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "hpsdUsingLatestFirmware",
		Desc: "The hpsd service is configured to use latest (i.e. ToT, unreleased) firmware",
		Contacts: []string{
			"dcallagh@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Impl: &hpsdCustomFirmwareFixture{
			firmwarePath: hpsutil.LatestFirmwarePath,
		},
		// Need to allow 3 minutes for reboot during MakeRootfsWritable
		SetUpTimeout:    4 * time.Minute,
		TearDownTimeout: 5 * time.Second,
	})
}

type hpsdCustomFirmwareFixture struct {
	// Path to custom firmware on the DUT filesystem.
	// hpsd will be reconfigured to load firmware from this path.
	firmwarePath string
}

func (f *hpsdCustomFirmwareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	dut := s.DUT()

	// Check that the requested firmware actually exists on disk on the DUT.
	if err := dut.Conn().CommandContext(ctx, "stat", f.firmwarePath).Run(); err != nil {
		s.Fatalf("Failed to stat custom firmware at %v (it might be missing): %v",
			f.firmwarePath, err)
	}

	testing.ContextLog(ctx, "Ensuring rootfs is writable")
	if err := sysutil.MakeRootfsWritable(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to make rootfs writable: ", err)
	}

	testing.ContextLogf(ctx, "Copying original Upstart override for hpsd %v -> %v",
		overrideFilename, overrideOrigFilename)
	cmd := dut.Conn().CommandContext(ctx, "cp", "-p", overrideFilename, overrideOrigFilename)
	if err := cmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatalf("Failed to preserve %s: %v", overrideFilename, err)
	}

	hpsdFlags := []string{
		"--bus=/dev/i2c-hps-controller",
		fmt.Sprintf("--mcu_fw_image=%s/mcu_stage1.bin.xz", f.firmwarePath),
		fmt.Sprintf("--version_file=%s/mcu_stage1.version.txt", f.firmwarePath),
		fmt.Sprintf("--fpga_bitstream=%s/fpga_bitstream.bin.xz", f.firmwarePath),
		fmt.Sprintf("--fpga_app_image=%s/fpga_application.bin.xz", f.firmwarePath),
	}
	// TODO(dcallagh): these are not really "hardware" flags, should stop abusing this var
	overrideContents := fmt.Sprintf("env HPS_HW_FLAGS=\"%s\"", strings.Join(hpsdFlags, " "))

	testing.ContextLogf(ctx, "Writing Upstart override for hpsd at %v with contents: %v",
		overrideFilename, overrideContents)
	if err := linuxssh.WriteFile(ctx, dut.Conn(), overrideFilename, []byte(overrideContents), 0644); err != nil {
		s.Fatalf("Failed to write %s: %v", overrideFilename, err)
	}

	// Stop and start the hpsd service. We can't just restart it because
	// Upstart will not pick up the new command-line flags in that case.
	testing.ContextLog(ctx, "Stopping hpsd")
	if err := dut.Conn().CommandContext(ctx, "stop", "hpsd").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop hpsd: ", err)
	}
	testing.ContextLog(ctx, "Starting hpsd")
	if err := dut.Conn().CommandContext(ctx, "start", "hpsd").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to start hpsd: ", err)
	}

	return nil
}

func (f *hpsdCustomFirmwareFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	dut := s.DUT()

	testing.ContextLogf(ctx, "Restoring original Upstart override for hpsd %v -> %v",
		overrideOrigFilename, overrideFilename)
	cmd := dut.Conn().CommandContext(ctx, "mv", overrideOrigFilename, overrideFilename)
	if err := cmd.Run(ssh.DumpLogOnError); err != nil {
		s.Fatalf("Failed to restore %s: %v", overrideFilename, err)
	}

	// Stop and start the hpsd service. We can't just restart it because
	// Upstart will not pick up the new command-line flags in that case.
	testing.ContextLog(ctx, "Stopping hpsd")
	if err := dut.Conn().CommandContext(ctx, "stop", "hpsd").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop hpsd: ", err)
	}
	testing.ContextLog(ctx, "Starting hpsd")
	if err := dut.Conn().CommandContext(ctx, "start", "hpsd").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to start hpsd: ", err)
	}
}

func (f *hpsdCustomFirmwareFixture) Reset(ctx context.Context) error                        { return nil }
func (f *hpsdCustomFirmwareFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *hpsdCustomFirmwareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
