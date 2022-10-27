// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// rtd2142InfoPattern matches the expected output from fwupd when a
// RTD2142 is detected in the system.
const rtd2142InfoPattern = `.*RTD2142:
.*Device ID:          [0-9a-f]+
.*Summary:            DisplayPort MST hub
.*Current version:    \d+\.\d+
.*Vendor:             Realtek \(PCI:0x10EC\)
.*GUID:               15cb53a3-3217-5949-87ac-2e5cce94e15b \? I2C\\NAME_10EC2142:00
`

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdDetectRTD2142,
		Desc: "Checks that fwupd can detect realtek-mst devices",
		Contacts: []string{
			"pmarheine@chromium.org",    // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"fwupd"},
		HardwareDeps: hwdep.D(
			hwdep.DisplayPortConverter("RTD2142"),
		),
	})
}

func verifyRTD2142Detected(ctx context.Context, output []byte) error {
	matched, err := regexp.Match(rtd2142InfoPattern, output)
	if err != nil {
		return err
	}
	if !matched {
		outdir, ok := testing.ContextOutDir(ctx)
		if !ok {
			return errors.New("failed to get test out dir")
		}
		if err := ioutil.WriteFile(filepath.Join(outdir, "fwupd_output.txt"),
			output, 0644); err != nil {
			testing.ContextLogf(ctx, "Failed to write fwupd output to file: %s", err)
		}
		testing.ContextLog(ctx, "On some devices (particularly if in a"+
			" pre-MP build phase), the MST firmware may be too"+
			" old to support detection and needs to be replaced:"+
			" this failure may be a problem with the device the test"+
			" ran on, not a bug in this test or fwupd. See"+
			" https://issuetracker.google.com/issues/173742142#comment30"+
			" for details.")
		return errors.New("get-devices output didn't match expected format")
	}
	return nil
}

// FwupdDetectRTD2142 runs fwupdmgr and verifies that a RTD2142 is recognized.
func FwupdDetectRTD2142(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "get-devices")

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := verifyRTD2142Detected(ctx, output); err != nil {
		s.Fatal("fwupdmgr failed to detect RTD2142: ", err)
	}
}
