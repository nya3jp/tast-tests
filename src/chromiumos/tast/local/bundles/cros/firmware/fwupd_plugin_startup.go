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
		Func: FwupdPluginStartup,
		Desc: "Checks that the powerd plugin is enabled",
		Contacts: []string{
			"gpopoola@google.com",       // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// checkForPluginStr verifies through the output that a plugin was not disabled
func checkForPluginStr(output []byte, plugin string) error {
	var wp struct {
		Plugins []struct {
			Name  string
			Flags []string
		}
	}

	if err := json.Unmarshal(output, &wp); err != nil {
		return errors.New("failed to parse command output")
	}

	for _, p := range wp.Plugins {
		if p.Name == plugin {
			for _, f := range p.Flags {
				if f == "disabled" {
					return errors.New("plugin was found to be disabled")
				}
			}
			return nil
		}
	}

	return errors.New("plugin was not found")
}

// FwupdPluginStartup runs fwupdmgr get-plugins, retrieves the output, and
// checks that the expected plugins are enabled
func FwupdPluginStartup(ctx context.Context, s *testing.State) {
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

	for _, plugin := range []string{
		"analogix",
		"ccgx",
		"cros_ec",
		"dell_dock",
		"dfu",
		"emmc",
		"nvme",
		"parade_lspcon",
		"pixart_rf",
		"powerd",
		"realtek_mst",
		"synaptics_cxaudio",
		"synaptics_mst",
		"test",
		"thunderbolt",
		"vli",
		"wacom_raw",
		"wacom_usb",
	} {
		s.Run(ctx, plugin, func(ctx context.Context, s *testing.State) {
			if err := checkForPluginStr(output, plugin); err != nil {
				s.Fatal("search unsuccessful: ", err)
			}
		})
	}
}
