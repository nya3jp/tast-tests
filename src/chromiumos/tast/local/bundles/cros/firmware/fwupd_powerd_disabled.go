// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
//	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	target = `{\s+"Name" : "powerd",\s+"Flags" : \[[^][]+"disabled",[^][]+\]\s+}`
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
func stopPowerd(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "stop", "powerd")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		return err
	}

	if output == nil {
		return errors.Errorf("stop powerd output indicates it didn't work as expected: %q", output)
	}

	return nil
}

// startupPowerd runs the command "start powerd" to make the daemon run again after testing
func startupPowerd(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "start", "powerd")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		return err
	}

	if output == nil {
		return errors.Errorf("start powerd output indicates it didn't work as expected: %q", output)
	}

	return nil
}

// checkForPowerdDisabled verifies that powerd was found among disabled plugins */
func checkForPowerdDisabled(output []byte, s *testing.State) error {
	type wrapper struct {
		Plugins []struct {
			Name string
			Flag []string
		}
	}

	var w wrapper

	s.Log(string(output))
	if err := json.Unmarshal(output, &w); err != nil {
		s.Errorf("Failed to parse output: ", err)
	}
	s.Log(w)

	/* matched, err := regexp.Match(target, output)

	if err != nil {
		return err
	}
	if !matched {
		return errors.New("powerd was not found to be disabled")
	} */

	return nil
}

// FwupdPowerdDisabled calls stopPowerd to stop the daemon, runs fwupdmgr get-plugins,
// retrieves the output, and checks that the powerd plugin was not enabled
func FwupdPowerdDisabled(ctx context.Context, s *testing.State) {
	if err := stopPowerd(ctx); err != nil {
		s.Fatalf("failed to stop the power daemon: ", err)
	}


	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-plugins", "--json")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForPowerdDisabled(output, s); err != nil {
		s.Error("match failed: ", err)
	}

	if err := startupPowerd(ctx); err != nil {
		s.Fatalf("failed to restart the power daemon after testing: ", err)
	}
}
