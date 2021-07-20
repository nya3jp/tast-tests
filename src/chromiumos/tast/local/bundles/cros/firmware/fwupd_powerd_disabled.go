// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
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

// checkForPowerdDisabled verifies that powerd was found among disabled plugins */
func checkForPowerdDisabled(output []byte) error {
	type wrapper struct {
		Plugins []struct {
			Name  string
			Flags []string
		}
	}

	var w wrapper

	if err := json.Unmarshal(output, &w); err != nil {
		return errors.New("failed to parse command output")
	}

	for _, p := range w.Plugins {
		if p.Name == "powerd" {
			for _, f := range p.Flags {
				if f == "disabled" {
					return nil
				}
			}
		}
	}

	return errors.New("powerd was not found among plugins (to be disabled)")
}

// FwupdPowerdDisabled calls stopPowerd to stop the daemon, runs fwupdmgr get-plugins,
// retrieves the output, and checks that the powerd plugin was not enabled
func FwupdPowerdDisabled(ctx context.Context, s *testing.State) {
	// makes sure fwupd has started again after it stopped with powerd
	// to prevent risk of timeout from DBus activation
	if err := upstart.StopJob(ctx, "powerd"); err != nil {
		s.Fatal("Failed to stop the power daemon: ", err)
	}

	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
			s.Error("Failed to restart the power daemon after testing: ", err)
		}
	}()

	if err := upstart.EnsureJobRunning(ctx, "fwupd"); err != nil {
		s.Error("Failed to start fwupd (or ensure its already running): ", err)
	}

	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-plugins", "--json")

	output, err := cmd.Output(testexec.DumpLogOnError)

	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForPowerdDisabled(output); err != nil {
		s.Error("match failed: ", err)
	}

}
