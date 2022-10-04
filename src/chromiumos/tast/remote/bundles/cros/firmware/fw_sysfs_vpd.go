// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func generateRandomString() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	s := make([]rune, 8)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     FWSysfsVPD,
		Desc:     "Basic check for reading VPD data through sysfs",
		Contacts: []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		// TODO(b/194910939): Add back to firmware_unstable once this test actually works.
		Attr:         []string{},
		Fixture:      fixture.DevMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      20 * time.Minute,
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
	})
}

// FWSysfsVPD checks for VPD data integrity between reboots
// via reading from sysfs values and logs them on test output.
func FWSysfsVPD(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	// Check for FW version and fail if it's below 8846
	// See b:156407743 for details.
	s.Log("Checking for FW version")
	fwidOut, err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fwid").Output()
	if err != nil {
		s.Fatal("Failed to check firmware version: ", err)
	}

	fwidMajorStr := strings.Split(string(fwidOut), ".")

	fwidMajor, err := strconv.Atoi(fwidMajorStr[1])
	if err != nil {
		s.Fatal("Failed to parse firmware version string: ", string(fwidOut))
	}

	if fwidMajor <= 8846 {
		s.Fatal("Firmware version is below 8846, cannot continue: ", fwidMajor)
	}

	s.Log("Backing up current RW_VPD region for safety")
	rwvpdPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Programmer: pb.Programmer_BIOSProgrammer,
		Section:    pb.ImageSection_RWVPDImageSection,
	})
	if err != nil {
		s.Fatal("Failed to backup current RW_VPD region: ", err)
	}
	s.Log("RW_VPD region backup is stored at: ", rwvpdPath.Path)

	s.Log("Backing up current RO_VPD region for safety")
	rovpdPath, err := h.BiosServiceClient.BackupImageSection(ctx, &pb.FWSectionInfo{
		Programmer: pb.Programmer_BIOSProgrammer,
		Section:    pb.ImageSection_ROVPDImageSection,
	})
	if err != nil {
		s.Fatal("Failed to backup current RO_VPD region: ", err)
	}
	s.Log("RO_VPD region backup is stored at: ", rovpdPath.Path)

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

		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to ensure the DUT is booted")
		}

		s.Log("Restoring RW_VPD image")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, rwvpdPath); err != nil {
			s.Error("Failed to restore RW_VPD image: ", err)
		}

		s.Log("Restoring RO_VPD image")
		if _, err := h.BiosServiceClient.RestoreImageSection(ctx, rovpdPath); err != nil {
			s.Error("Failed to restore RO_VPD image: ", err)
		}

		s.Log("Removing VPD image backups from DUT")
		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", rwvpdPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete RW_VPD image from DUT: ", err)
		}

		if _, err := h.DUT.Conn().CommandContext(ctx, "rm", rovpdPath.Path).Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to delete RO_VPD image from DUT: ", err)
		}
	}(ctx)

	// Shorten the deadline for everything to save some time for the restore.
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()

	s.Log("Generating random strings for VPD values")
	roSectionString := generateRandomString()
	rwSectionString := generateRandomString()

	// Log out current VPD values.
	currentVPDROout, err := h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RO_VPD", "-l").Output()
	if err != nil {
		s.Fatal("Failed to read current RO_VPD value: ", err)
	}

	currentVPDRWout, err := h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RW_VPD", "-l").Output()
	if err != nil {
		s.Fatal("Failed to read current RW_VPD value: ", err)
	}

	s.Log("Current RO_VPD values: ", string(currentVPDROout))
	s.Log("Current RW_VPD values: ", string(currentVPDRWout))

	s.Log("RW_TEST value: ", rwSectionString)
	s.Log("RO_TEST value: ", roSectionString)

	s.Log("Writing RO_VPD value")
	err = h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RO_VPD", "-s", fmt.Sprintf("RO_TEST=%s", roSectionString)).Run()
	if err != nil {
		s.Fatal("Failed to write random RO_VPD value: ", err)
	}

	s.Log("Writing RW_VPD value")
	err = h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RW_VPD", "-s", fmt.Sprintf("RW_TEST=%s", rwSectionString)).Run()
	if err != nil {
		s.Fatal("Failed to write random RW_VPD value: ", err)
	}

	s.Log("Rebooting DUT")
	h.CloseRPCConnection(ctx)
	if err := h.DUT.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Verifying RO_VPD value")
	var newVPDROout []byte
	newVPDROout, err = h.DUT.Conn().CommandContext(ctx, "cat", "/sys/firmware/vpd/ro/RO_TEST").Output()
	if err != nil {
		s.Fatal("Failed to read new RO_VPD value after a reboot: ", err)
	}

	if string(newVPDROout) != roSectionString {
		s.Fatalf("RO_VPD section mismatch! (expected: %s, got: %s)", roSectionString, newVPDROout)
	}

	s.Log("Verifying RW_VPD value")
	var newVPDRWout []byte
	newVPDRWout, err = h.DUT.Conn().CommandContext(ctx, "cat", "/sys/firmware/vpd/rw/RW_TEST").Output()
	if err != nil {
		s.Fatal("Failed to read new RW_VPD value after a reboot: ", err)
	}

	if string(newVPDRWout) != rwSectionString {
		s.Fatalf("RO_VPD section mismtch! (expected: %s, got: %s)", rwSectionString, newVPDRWout)
	}

	s.Log("Removing keys form RW_VPD")
	err = h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RW_VPD", "-d", "RW_TEST").Run()
	if err != nil {
		s.Fatal("Failed to delete random RW_VPD value: ", err)
	}

	s.Log("Removing keys from RO_VPD")
	err = h.DUT.Conn().CommandContext(ctx, "vpd", "-i", "RO_VPD", "-d", "RO_TEST").Run()
	if err != nil {
		s.Fatal("Failed to delete random RO_VPD value: ", err)
	}
}
