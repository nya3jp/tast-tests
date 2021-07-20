// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
)

const (
	target = `.*powerd:\s{1,}Flags:.*Disabled`
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdPowerdDisabled,
		Desc: "Checks that the powerd plugin is disabled in the absense of powerd",
		Contacts: []string{
			"gpopoola@google.com",       // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// stopPowerd runs the command "stop powerd" to halt the daemon
func stopPowerd(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "stop", "powerd")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if (err != nil) || (output == nil) {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}

// restartPowerd runs the command "start powerd" to restart the daemon after testing
func restartPowerd(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "start", "powerd")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if (err != nil) || (output == nil) {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}

// checkForPowerdDis verifies that powerd was found among disabled plugins */
func checkForPowerdDis(output []byte) error {
	matched, err := regexp.Match(targetStr, output)
	if err != nil {
		return err
	}
	if !matched {
		return errors.New("powerd was not found to be disabled")
	}
	return nil
}

// FwupdPowerdDiasbaled calls stopPowerd to stop the daemon, runs fwupdmgr get-plugins,
// retrieves the output, and checks that the powerd plugin was not enabled
func FwupdPowerdDisabled(ctx context.Context, s *testing.State) {
	stopPowerd(ctx, s)

	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-plugins")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Fatal("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForPowerdStr(output); err != nil {
		s.Fatal("powerd was not diabled: ", err)
	}

	restartPowerd(ctx, s)
}
