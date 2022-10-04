// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	fwpb "chromiumos/tast/services/cros/firmware"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Used to match the RO/RW versions reported by EC Tool, as declared here:
// https://chromium.googlesource.com/chromiumos/platform/ec/+/main/util/ectool.c
var (
	reROVersion = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion = regexp.MustCompile(`RW version:\s*(\S+)\s`)
)

var (
	ecPathVar = testing.RegisterVarString(
		"firmware.ECPath",
		"$HOME/chromiumos/src/platform/ec/ec.bin",
		"It takes EC path as input for e.g. /path/to/ec.bin")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableEnableECSWSync,
		Desc:         "Flash EC using flashrom and enable disable EC SW sync",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		Fixture:      fixture.NormalMode,
	})
}

func DisableEnableECSWSync(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	pathToEC := ecPathVar.Value()

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient

	s.Log("Backing up current EC_RW region for safety")
	ecPath, err := h.BiosServiceClient.BackupImageSection(ctx, &fwpb.FWSectionInfo{
		Programmer: fwpb.Programmer_ECProgrammer,
		Section:    fwpb.ImageSection_ECRWImageSection,
	})
	if err != nil {
		s.Fatal("Failed to backup current EC_RW region: ", err)
	}
	s.Log("EC_RW region backup is stored at: ", ecPath.Path)

	defer func(ctx context.Context) {
		s.Log("Wait for DUT to reconnect")
		if err = h.DUT.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		s.Log("Reconnecting to RPC services on DUT")
		if err := h.RequireRPCClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to the RPC service on DUT: ", err)
		}

		s.Log("Reconnecting to BiosService on DUT")
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to reconnect to BiosServiceClient on DUT: ", err)
		}

		s.Log("Restoring EC image")
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to ensure the DUT is booted")
		}
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, ecPath); err != nil {
			s.Error("Failed to restore EC image: ", err)
		}
		s.Log("Removing EC image backup from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", ecPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete EC image from DUT: ", err)
		}
	}(ctx)

	paths := strings.Split(pathToEC, "/")
	ecPathLocal := filepath.Join("/usr/local", paths[len(paths)-1])

	ecKeyPath := map[string]string{pathToEC: ecPathLocal}
	if _, err := linuxssh.PutFiles(ctx, h.DUT.Conn(), ecKeyPath, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", ecPathLocal, err)
	}

	ro1, rw1, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro1 != rw1 {
		s.Fatalf("Failed to verify EC version, versions should be the same, got: RO=%s, RW=%s, want: RO == RW", ro1, rw1)
	}

	s.Log("Clearing GBB flag DISABLE_EC_SOFTWARE_SYNC")
	clear := pb.GBBFlagsState{Clear: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}}
	if _, err := bs.ClearAndSetGBBFlags(ctx, &clear); err != nil {
		s.Fatal("Failed to clear GBB flag: ", err)
	}

	s.Log("Rebooting DUT")
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	ro2, rw2, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro1 != ro2 || rw1 != rw2 {
		s.Fatalf("Failed to verify EC versions, versions should be the same, got %s=%s (RO), %s=%s (RW)", ro1, ro2, rw1, rw2)
	}

	if err := flashEC(ctx, h.DUT, ecPathLocal); err != nil {
		s.Fatal("Failed to flash EC: ", err)
	}

	s.Log("Cold Reboot DUT")
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	ro3, rw3, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro3 == ro2 || rw3 == rw2 {
		s.Fatalf("Failed to verify EC version, EC should change as per flashed EC binary got (%s, %s) RO (%s, %s) RW", ro2, ro3, rw2, rw3)
	}

	s.Log("Setting GBB flag DISABLE_EC_SOFTWARE_SYNC")
	set := pb.GBBFlagsState{Set: []pb.GBBFlag{pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC}}
	if _, err := bs.ClearAndSetGBBFlags(ctx, &set); err != nil {
		s.Fatal("Failed to clear GBB flag: ", err)
	}

	s.Log("Rebooting DUT")
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	ro4, rw4, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro4 == rw4 && ro4 != ro3 {
		s.Fatalf("Failed to verify EC version, RW and RO regions of EC should not be the same version got %s=%s (RO, RW), RO version of EC should be version of EC flashed got %s=%s (RO, RO)", ro4, rw4, ro3, ro4)
	}

}

// checkECVersion returns ro/rw version from ectool.
func checkECVersion(ctx context.Context, dut *dut.DUT) (string, string, error) {
	ec := firmware.NewECTool(dut, firmware.ECToolNameMain)
	output, err := ec.Command(ctx, "version").Output(ssh.DumpLogOnError)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to run 'ectool version' on DUT")
	}
	roVersion := reROVersion.FindSubmatch(output)
	if len(roVersion) == 0 {
		return "", "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reROVersion, output)
	}
	rwVersion := reRWVersion.FindSubmatch(output)
	if len(rwVersion) == 0 {
		return "", "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reRWVersion, output)
	}
	return string(roVersion[1]), string(rwVersion[1]), nil
}

// flashEC flashes DUT using flashrom.
func flashEC(ctx context.Context, dut *dut.DUT, imagePath string) error {
	testing.ContextLogf(ctx, "Writing image from file %s", imagePath)
	args := []string{"-p", "ec", "-w", imagePath}
	if out, err := dut.Conn().CommandContext(ctx, "flashrom", args...).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce success message: %s", string(out))
	}
	return nil
}
