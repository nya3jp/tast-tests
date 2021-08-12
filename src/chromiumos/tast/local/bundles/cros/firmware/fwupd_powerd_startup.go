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
		Func: FwupdPowerdStartup,
		Desc: "Checks that the powerd plugin is enabled",
		Contacts: []string{
			"gpopoola@google.com",       // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// checkForPowerdStr verifies that powerd was found among enabled plugins
func checkForPowerdStr(output []byte) error {
	type wrapper struct {
		Plugins []struct {
			Name  string
			Flags []string
		}
	}
	var wp wrapper

	if err := json.Unmarshal(output, &wp); err != nil {
		return errors.New("failed to parse command output")
	}

	for _, p := range wp.Plugins {
		if p.Name == "powerd" {
			for _, f := range p.Flags {
				if f == "disabled" {
					return errors.New("plugin was found to be disabled")
				}
			}
		}
	}

	return nil
}

// FwupdPowerdStartup runs fwupdmgr get-plugins, retrieves the output, and
// checks for powerd
func FwupdPowerdStartup(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "fwupd"); err != nil {
		s.Fatal("Failed to restart fwupd: ", err)
	}

	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-plugins", "--json")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
		s.Error("Failed dumping fwupdmgr output: ", err)
	}

	if err := checkForPowerdStr(output); err != nil {
		s.Fatal("search unsuccessful: ", err)
	}
}
