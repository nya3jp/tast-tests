// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"path"
	"strings"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetSysfsData,
		Desc: "Test sending GetSysfsData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetSysfsData(ctx context.Context, s *testing.State) {
	getSysfsData := func(ctx context.Context, s *testing.State, typeField dtcpb.GetSysfsDataRequest_Type, expectedPrefix string, expectedFiles []string) {
		request := dtcpb.GetSysfsDataRequest{
			Type: typeField,
		}
		response := dtcpb.GetSysfsDataResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
			s.Fatal("Unable to get Sysfs files: ", err)
		}

		if len(response.FileDump) == 0 {
			s.Fatal("No file dumps available")
		}

		for _, dump := range response.FileDump {
			if !strings.HasPrefix(dump.Path, expectedPrefix) {
				s.Errorf("File %s does not have prefix %s", dump.Path, expectedPrefix)
			}

			// Sanity check, this should not happen
			if dump.CanonicalPath == "" {
				s.Errorf("File %s has an empty cannonical path", dump.Path)
			}

			// dump.Contents is empty for some of the files in sysfs so we perform
			// no check.
		}

		filePresent := func(path string) bool {
			for _, dump := range response.FileDump {
				if dump.Path == path {
					return true
				}
			}

			return false
		}

		for _, expectedFile := range expectedFiles {
			expectedFile = path.Join(expectedPrefix, expectedFile)

			if !filePresent(expectedFile) {
				s.Errorf("Expected path %s not found", expectedFile)
			}
		}
	}

	for _, param := range []struct {
		// name is the subtest name
		name string
		// typeField is sent as the request type to GetSysfsData.
		typeField dtcpb.GetSysfsDataRequest_Type
		// expectedPrefix is a prefix that all returned paths must have.
		expectedPrefix string
		// expectedFiles contains a list of files expected to be present. All the
		// files in this list must be present, but additional ones can exist. They
		// are relative to expectedPrefix.
		expectedFiles []string
	}{
		{
			name:           "hwmon",
			typeField:      dtcpb.GetSysfsDataRequest_CLASS_HWMON,
			expectedPrefix: "/sys/class/hwmon/",
		}, {
			name:           "thermal",
			typeField:      dtcpb.GetSysfsDataRequest_CLASS_THERMAL,
			expectedPrefix: "/sys/class/thermal/",
		}, {
			name:           "firmware_dmi_table",
			typeField:      dtcpb.GetSysfsDataRequest_FIRMWARE_DMI_TABLES,
			expectedPrefix: "/sys/firmware/dmi/tables/",
			expectedFiles:  []string{"DMI", "smbios_entry_point"},
		}, {
			name:           "power_supply",
			typeField:      dtcpb.GetSysfsDataRequest_CLASS_POWER_SUPPLY,
			expectedPrefix: "/sys/class/power_supply/",
		}, {
			name:           "backlight",
			typeField:      dtcpb.GetSysfsDataRequest_CLASS_BACKLIGHT,
			expectedPrefix: "/sys/class/backlight/",
		}, {
			name:           "network",
			typeField:      dtcpb.GetSysfsDataRequest_CLASS_NETWORK,
			expectedPrefix: "/sys/class/net/",
		}, {
			name:           "cpu",
			typeField:      dtcpb.GetSysfsDataRequest_DEVICES_SYSTEM_CPU,
			expectedPrefix: "/sys/devices/system/cpu/",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			getSysfsData(ctx, s, param.typeField, param.expectedPrefix, param.expectedFiles)
		})
	}
}
