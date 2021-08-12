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
				Name: "solokey",
				Val:  "solokey",
			}, {
				Name: "system76_launch",
				Val:  "system76_launch",
			}, {
				Name: "parade_lspcon",
				Val:  "parade_lspcon",
			}, {
				Name: "fresco_pd",
				Val:  "fresco_pd",
			}, {
				Name: "synaptics_rmi",
				Val:  "synaptics_rmi",
			}, {
				Name: "superio",
				Val:  "superio",
			}, {
				Name: "synaptics_mst",
				Val:  "synaptics_mst",
			}, {
				Name: "pci_mei",
				Val:  "pci_mei",
			}, {
				Name: "hailuck",
				Val:  "hailuck",
			}, {
				Name: "synaptics_prometheus",
				Val:  "synaptics_prometheus",
			}, {
				Name: "fastboot",
				Val:  "fastboot",
			}, {
				Name: "thunderbolt",
				Val:  "thunderbolt",
			}, {
				Name: "wacom_raw",
				Val:  "wacom_raw",
			}, {
				Name: "cpu",
				Val:  "cpu",
			}, {
				Name: "altos",
				Val:  "altos",
			}, {
				Name: "nvme",
				Val:  "nvme",
			}, {
				Name: "ebitdo",
				Val:  "ebitdo",
			}, {
				Name: "dfu_csr",
				Val:  "dfu_csr",
			}, {
				Name: "rts54hid",
				Val:  "rts54hid",
			}, {
				Name: "optionrom",
				Val:  "optionrom",
			}, {
				Name: "test_ble",
				Val:  "test_ble",
			}, {
				Name: "vli",
				Val:  "vli",
			}, {
				Name: "test",
				Val:  "test",
			}, {
				Name: "linux_tainted",
				Val:  "linux_tainted",
			}, {
				Name: "cros_ec",
				Val:  "cros_ec",
			}, {
				Name: "pixart_rf",
				Val:  "pixart_rf",
			}, {
				Name: "logitech_hidpp",
				Val:  "logitech_hidpp",
			}, {
				Name: "dfu",
				Val:  "dfu",
			}, {
				Name: "thelio_io",
				Val:  "thelio_io",
			}, {
				Name: "linux_swap",
				Val:  "linux_swap",
			}, {
				Name: "ata",
				Val:  "ata",
			}, {
				Name: "elantp",
				Val:  "elantp",
			}, {
				Name: "acpi_dmar",
				Val:  "acpi_dmar",
			}, {
				Name: "redfish",
				Val:  "redfish",
			}, {
				Name: "linux_sleep",
				Val:  "linux_sleep",
			}, {
				Name: "realtek_mst",
				Val:  "realtek_mst",
			}, {
				Name: "invalid",
				Val:  "invalid",
			}, {
				Name: "pci_bcr",
				Val:  "pci_bcr",
			}, {
				Name: "ccgx",
				Val:  "ccgx",
			}, {
				Name: "wacom_usb",
				Val:  "wacom_usb",
			}, {
				Name: "acpi_facp",
				Val:  "acpi_facp",
			}, {
				Name: "rts54hub",
				Val:  "rts54hub",
			}, {
				Name: "iommu",
				Val:  "iommu",
			}, {
				Name: "jabra",
				Val:  "jabra",
			}, {
				Name: "synaptics_cxaudio",
				Val:  "synaptics_cxaudio",
			}, {
				Name: "goodixmoc",
				Val:  "goodixmoc",
			}, {
				Name: "analogix",
				Val:  "analogix",
			}, {
				Name: "steelseries",
				Val:  "steelseries",
			}, {
				Name: "ep963x",
				Val:  "ep963x",
			}, {
				Name: "linux_lockdown",
				Val:  "linux_lockdown",
			}, {
				Name: "upower",
				Val:  "upower",
			}, {
				Name: "bcm57xx",
				Val:  "bcm57xx",
			}, {
				Name: "colorhug",
				Val:  "colorhug",
			}, {
				Name: "acpi_phat",
				Val:  "acpi_phat",
			}, {
				Name: "dell_dock",
				Val:  "dell_dock",
			}, {
				Name: "nitrokey",
				Val:  "nitrokey",
			}, {
				Name: "emmc",
				Val:  "emmc",
			}, {
				Name: "msr",
				Val:  "msr",
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
