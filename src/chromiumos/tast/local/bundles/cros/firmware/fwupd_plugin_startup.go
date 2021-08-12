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
		Params: []testing.Param{
			{
				Name: "powerd",
				Val:  "powerd",
			}, {
				Name: "parade_lspcon",
				Val:  "parade_lspcon",
			}, {
				Name: "synaptics_mst",
				Val:  "synaptics_mst",
			}, {
				Name: "thunderbolt",
				Val:  "thunderbolt",
			}, {
				Name: "wacom_raw",
				Val:  "wacom_raw",
			}, {
				Name: "nvme",
				Val:  "nvme",
			}, {
				Name: "vli",
				Val:  "vli",
			}, {
				Name: "test",
				Val:  "test",
			}, {
				Name: "cros_ec",
				Val:  "cros_ec",
			}, {
				Name: "pixart_rf",
				Val:  "pixart_rf",
			}, {
				Name: "dfu",
				Val:  "dfu",
			}, {
				Name: "realtek_mst",
				Val:  "realtek_mst",
			}, {
				Name: "ccgx",
				Val:  "ccgx",
			}, {
				Name: "wacom_usb",
				Val:  "wacom_usb",
			}, {
				Name: "synaptics_cxaudio",
				Val:  "synaptics_cxaudio",
			}, {
				Name: "analogix",
				Val:  "analogix",
			}, {
				Name: "dell_dock",
				Val:  "dell_dock",
			}, {
				Name: "emmc",
				Val:  "emmc",
			}},
	})
}

// checkForPluginStr verifies through the output that a plugin was not disabled
func checkForPluginStr(output []byte, s *testing.State) error {
	plugin := s.Param().(string)

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
		if p.Name == plugin {
			for _, f := range p.Flags {
				if f == "disabled" {
					return errors.New("plugin was found to be disabled")
				}
			}
		}
	}

	return nil
}

// FwupdPluginStartup runs fwupdmgr get-plugins, retrieves the output, and
// checks that a plugin was enabled
func FwupdPluginStartup(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "fwupd"); err != nil {
		s.Fatal("Failed to restart fwupd: ", err)
	}

	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-plugins", "--json")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "fwupdmgr.txt"), output, 0644); err != nil {
			s.Error("Failed dumping fwupdmgr output: ", err)
		}
	}

	if err := checkForPluginStr(output, s); err != nil {
		s.Fatal("search unsuccessful: ", err)
	}
}
