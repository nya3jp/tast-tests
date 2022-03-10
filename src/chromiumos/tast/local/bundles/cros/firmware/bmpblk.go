// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Bmpblk,
		Desc: "Inspects coreboot contents for configuration indicators for the bitmaps used in firmware UI",
		Contacts: []string{
			"jwerner@chromium.org",       // Test author
			"kmshelton@chromium.org",     // Test porter (from TAuto)
			"chromeos-faft@chromium.org", // Backup mailing list
		},
		// TODO: Move to firmware_unstable, then firmware_bios
		Attr:        []string{"group:firmware", "firmware_experimental"},
		ServiceDeps: []string{"tast.cros.firmware.BiosService"},
	})
}

//Bmpblk inspects coreboot contents.  See https://chromium.googlesource.com/chromiumos/platform/bmpblk for more context.
func Bmpblk(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient

	coreboot, err := bs.BackupImageSection(ctx, &pb.FWBackUpSection{
		Programmer: pb.Programmer_BIOSProgrammer,
		Section:    pb.ImageSection_BOOTSTUBImageSection,
	})
	if err != nil {
		s.Fatal("Failed to backup current BOOTSTUB region: ", err)
	}
	s.Log("BOOTSTUB region (which should contain coreboot) backup is stored at: ", coreboot.Path)

	cmd := testexec.CommandContext(ctx, "cbfstool "+coreboot.Path+" -r BOOT_STUB")
	validityRe := regexp.MustCompile(`romstage`)
	naRe := regexp.MustCompile(`vbgfx.bin`)
	notScaledRe := regexp.MustCompile(`vbgfx_not_scaled`)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else {
		outs := string(out)
		path := filepath.Join(s.OutDir(), "cbfs.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save cbfstool output: ", err)
		}
		if !validityRe.MatchString(outs) {
			s.Fatalf("Failed validity check.  Output of %q did not contain %q (saved output to %s)",
				shutil.EscapeSlice(cmd.Args), validityRe, filepath.Base(path))
		} else if naRe.MatchString(outs) {
			s.Log("This board appears to have no firmware screens")
			return
		} else if !notScaledRe.MatchString(outs) {
			s.Fatalf("Failed inspection for generic configuration.  This bmpblk was configured for a generic "+
				"1366x768 display resolution.  Images will get scaled up at runtime and look blurry.  You need to "+
				"explicitly set the panel resolution for this board in bmpblk/images/boards.yaml and add it to "+
				"CROS_BOARDS in the sys-boot/chromeos-bmpblk .ebuild.  Do *not* do this until you are certain of "+
				"the panel resolution that the final product will ship with!  Output of %q did not contain %q (saved"+
				" output to %s).", shutil.EscapeSlice(cmd.Args), notScaledRe, filepath.Base(path))
		}
	}
}
