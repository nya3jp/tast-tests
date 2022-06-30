// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlashCorebootFlashrom,
		Desc:         "Coreboot: Flash newer coreboot version using flashrom",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"flashrom", "chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Vars:         []string{"firmware.CBPath"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func FlashCorebootFlashrom(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper
	dut := s.DUT()

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	output, err := h.Reporter.CatFile(ctx, "/sys/firmware/log")
	if err != nil {
		s.Fatal("Failed to read firmware log: ", err)
	}
	me := regexp.MustCompile(`ME:\s*Manufacturing Mode\s*:\s*(\S+)\s`)
	match := me.FindSubmatch([]byte(output))
	if strings.TrimSpace(string((match[1]))) == "NO" {
		s.Fatal("Flashing using flashrom command will fail")
	}

	pathtoCB := s.RequiredVar("firmware.CBPath")

	paths := strings.Split(pathtoCB, "/")
	cbPathLocal := filepath.Join("/usr/local", paths[len(paths)-1])

	cbKeyPath := map[string]string{pathtoCB: cbPathLocal}
	if _, err := linuxssh.PutFiles(ctx, h.DUT.Conn(), cbKeyPath, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", cbPathLocal, err)
	}

	fwidBefore, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fwid").Output()
	if err != nil {
		s.Fatal("Failed to check firmware version: ", err)
	}

	if err := flashCoreboot(ctx, h.DUT, cbPathLocal); err != nil {
		s.Fatal("Failed to flash Coreboot: ", err)
	}

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, h.ServoProxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	h.CloseRPCConnection(ctx)
	powerState := "S5"
	if err := powercontrol.ShutdownAndWaitForPowerState(ctx, h.ServoProxy, dut, powerState); err != nil {
		s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
	}

	if err := powercontrol.PowerOntoDUT(ctx, h.ServoProxy, dut); err != nil {
		s.Fatal("Failed to wake up DUT: ", err)
	}

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	s.Log("Logging in to Chrome")
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	fwidAfter, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fwid").Output()
	if err != nil {
		s.Fatal("Failed to check firmware version: ", err)
	}

	if string(fwidAfter) == string(fwidBefore) {
		s.Fatal("Failed to verify coreboot, version is not updated")
	}

}

func flashCoreboot(ctx context.Context, dut *dut.DUT, imagePath string) error {
	testing.ContextLogf(ctx, "Writing image from file %s", imagePath)
	args := []string{"-p", string(bios.HostProgrammer), "-w", imagePath}
	if out, err := dut.Conn().CommandContext(ctx, "flashrom", args...).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run flashrom cmd")
	} else if match := regexp.MustCompile(`SUCCESS`).FindSubmatch(out); match == nil {
		return errors.Errorf("flashrom did not produce success message: %s", string(out))
	}
	return nil
}
