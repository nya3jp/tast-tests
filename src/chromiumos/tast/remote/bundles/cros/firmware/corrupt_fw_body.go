// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CorruptFWBody,
		Desc:         "Servo based firmware body corruption test",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental", "firmware_usb"},
		Timeout:      20 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		SoftwareDeps: []string{"crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "a_normal_mode",
				Fixture: fixture.NormalMode,
				Val:     "A",
			},
			{
				Name:    "b_normal_mode",
				Fixture: fixture.NormalMode,
				Val:     "B",
			},
			{
				Name:    "a_dev_mode",
				Fixture: fixture.DevModeGBB,
				Val:     "A",
			},
			{
				Name:    "b_dev_mode",
				Fixture: fixture.DevModeGBB,
				Val:     "B",
			},
		},
	})
}

func CorruptFWBody(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	fwVariant := s.Param().(string)
	var fwVariantOpposite string
	if fwVariant == "A" {
		fwVariantOpposite = "B"
	} else {
		fwVariantOpposite = "A"
	}

	var sectionVariant pb.ImageSection
	if strings.Contains(fwVariant, "A") {
		sectionVariant = pb.ImageSection_FWBodyAImageSection
	} else {
		sectionVariant = pb.ImageSection_FWBodyBImageSection
	}

	s.Log("Backup firmware body")
	FWBodyBkp, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{Section: sectionVariant, Programmer: pb.Programmer_BIOSProgrammer})
	if err != nil {
		s.Fatal("Failed to backup current FW Body region: ", err)
	}

	defer func() {
		s.Log("Delete FW Body backup")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", FWBodyBkp.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete FW Body backup: ", err)
		}
	}()

	s.Log("Set the USB Mux direction to Host")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal(err, "failed to set the USB Mux direction to the Host")
	}

	if err := fwUtils.ChangeFWVariant(ctx, h, ms, fwCommon.RWSectionA); err != nil {
		s.Fatal("Failed to change FW variant: ", err)
	}

	// Restore FW Body
	defer func() {
		// Disable wp so backup can be restored.
		if err := fwUtils.SetFWWriteProtect(ctx, h, false); err != nil {
			s.Fatal("Failed to set FW write protect state: ", err)
		}

		if err := h.RequireServo(ctx); err != nil {
			s.Fatal("Failed to init servo: ", err)
		}

		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Requiring BiosServiceClient: ", err)
		}

		s.Log("Restore firmware body")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, FWBodyBkp); err != nil {
			s.Fatal("Failed to restore FW Body: ", err)
		}

		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			s.Fatal("Failed to perform mode aware reboot: ", err)
		}

		vboot2, err := h.Reporter.Vboot2(ctx)
		if err != nil {
			s.Fatal("Failed to determine fw_vboot2: ", err)
		}

		var finalFWVer string
		if vboot2 {
			finalFWVer = fwVariantOpposite
		} else {
			finalFWVer = "A"
		}

		if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, finalFWVer); err != nil {
			s.Fatal(err, "failed to check a firmware version")
		} else if !isFWVerCorrect {
			s.Fatal("Failed to boot into the opposite firmware")
		}

	}()

	s.Log("Corrupt Firmware Body")
	if _, err := h.BiosServiceClient.CorruptFWSection(ctx, &pb.FWSectionInfo{Section: sectionVariant, Programmer: pb.Programmer_BIOSProgrammer}); err != nil {
		s.Fatalf("Failed to corrupt Firmware Body %s section: %v", fwVariant, err)
	}

	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	s.Log("Check the firmware version")
	if isFWVerOpp, err := h.Reporter.CheckFWVersion(ctx, fwVariantOpposite); err != nil {
		s.Fatal(err, "failed to check a firmware version")
	} else if !isFWVerOpp {
		s.Fatal("Failed to boot into the opposite firmware")
	}

	if fwVariant == "B" {
		s.Log("Set FW tries to B")
		if err := firmware.SetFWTries(ctx, h.DUT, fwCommon.RWSectionB, 0); err != nil {
			s.Fatal("Failed to set FW tries to B")
		}

		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			s.Fatal("Failed to perform mode aware reboot: ", err)
		}

		s.Log("Check the firmware version")
		if isFWVerOpp, err := h.Reporter.CheckFWVersion(ctx, fwVariantOpposite); err != nil {
			s.Fatal(err, "failed to check a firmware version")
		} else if !isFWVerOpp {
			s.Fatal("Failed to boot into the opposite firmware")
		}
	}

}
