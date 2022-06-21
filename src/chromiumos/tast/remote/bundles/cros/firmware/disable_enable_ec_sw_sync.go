// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/exec"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var (
	reROVersion = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion = regexp.MustCompile(`RW version:\s*(\S+)\s`)
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableEnableECSWSync,
		Desc:         "Flash EC using flashrom and enable disable EC SW sync",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		Vars:         []string{"firmware.ECPath"},
		Fixture:      fixture.NormalMode,
	})
}

func DisableEnableECSWSync(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	pathtoEC := s.RequiredVar("firmware.ECPath")

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	s.Log("Backing up current EC_RW region for safety")
	ecPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWBackUpSection{
		Programmer: pb.Programmer_ECProgrammer,
		Section:    pb.ImageSection_ECRWImageSection,
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

	paths := strings.Split(pathtoEC, "/")
	ecPathLocal := filepath.Join("/usr/local", paths[len(paths)-1])

	ecKeyPath := map[string]string{pathtoEC: ecPathLocal}
	if _, err := linuxssh.PutFiles(ctx, h.DUT.Conn(), ecKeyPath, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", ecPathLocal, err)
	}

	ro1, rw1, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro1 != rw1 {
		s.Fatal("Failed to verify EC version, RO & RW version of EC should be same")
	}

	DisableECSoftwareSync := "0x200"
	s.Logf("Setting GBB flag to %s", DisableECSoftwareSync)
	if err := h.DUT.Conn().CommandContext(ctx, "/usr/share/vboot/bin/set_gbb_flags.sh", DisableECSoftwareSync).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set GBB flags: ", err)
	}

	s.Log("Rebooting DUT")
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	ro2, rw2, err := checkECVersion(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to read ectool version: ", err)
	}
	if ro2 != rw2 || ro1 != ro2 || rw1 != rw2 {
		s.Fatal("Failed to verify EC version, EC version should be same and RO & RW version of EC should be same as old EC version")
	}

	if err := flashEC(ctx, h.DUT, ecPathLocal); err != nil {
		s.Fatal("Failed to flash ec: ", err)
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
		s.Fatal("Failed to verify EC version, EC should change as per flashed EC binary")
	}

	EnableECSoftwareSync := "0x0"
	s.Logf("Setting GBB flag to %s", EnableECSoftwareSync)
	if err := h.DUT.Conn().CommandContext(ctx, "/usr/share/vboot/bin/set_gbb_flags.sh", EnableECSoftwareSync).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set GBB flags: ", err)
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
		s.Fatal("Failed to verify EC version, RW and RO regions of EC should not be the same version, RO version of EC should be version of EC flashed")
	}

}

func checkECVersion(ctx context.Context, dut *dut.DUT) (string, string, error) {
	ec := firmware.NewECTool(dut, firmware.ECToolNameMain)
	output, err := ec.Command(ctx, "version").Output(ssh.DumpLogOnError)
	if err != nil {
		return "", "", errors.Wrap(err, "running 'ectool version' on DUT")
	}
	roVersion := reROVersion.FindSubmatch(output)
	if len(roVersion) == 0 {
		return "", "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reROVersion, output)
	}
	rwVersion := reRWVersion.FindSubmatch(output)
	if len(rwVersion) == 0 {
		return "", "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reROVersion, output)
	}
	return string(roVersion[1]), string(rwVersion[1]), nil
}

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
