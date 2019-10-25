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

// getSysfsDataParam is the parameter to the APIGetSysfsData test.
type getSysfsDataParam struct {
	// typeField is sent as the request type to GetSysfsData.
	typeField dtcpb.GetSysfsDataRequest_Type
	// expectedPrefix is a prefix that all returned paths must have.
	expectedPrefix string
	// expectedFiles contains a list of files expected to be present. They are relative to expectedPrefix.
	expectedFiles []string
}

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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
		Params: []testing.Param{{
			Name: "hwmon",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_HWMON,
				expectedPrefix: "/sys/class/hwmon/",
			},
		}, {
			Name: "thermal",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_THERMAL,
				expectedPrefix: "/sys/class/thermal/",
			},
		}, {
			Name: "firmware_dmi_table",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_FIRMWARE_DMI_TABLES,
				expectedPrefix: "/sys/firmware/dmi/tables/",
				expectedFiles:  []string{"DMI", "smbios_entry_point"},
			},
		}, {
			Name: "power_supply",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_POWER_SUPPLY,
				expectedPrefix: "/sys/class/power_supply/",
			},
		}, {
			Name: "backlight",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_BACKLIGHT,
				expectedPrefix: "/sys/class/backlight/",
			},
		}, {
			Name: "network",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_NETWORK,
				expectedPrefix: "/sys/class/net/",
			},
		}, {
			Name: "cpu",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_DEVICES_SYSTEM_CPU,
				expectedPrefix: "/sys/devices/system/cpu/",
			},
		}},
	})
}

func APIGetSysfsData(ctx context.Context, s *testing.State) {
	param := s.Param().(getSysfsDataParam)

	request := dtcpb.GetSysfsDataRequest{
		Type: param.typeField,
	}
	response := dtcpb.GetSysfsDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
		s.Fatal("Unable to get Sysfs files: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(response.FileDump) == 0 {
		s.Fatal("No file dumps available")
	}

	if param.expectedFiles != nil {
		for _, expectedFile := range param.expectedFiles {
			expectedFile = path.Join(param.expectedPrefix, expectedFile)
			found := false
			for _, dump := range response.FileDump {
				if dump.Path == expectedFile {
					found = true
					break
				}
			}

			if !found {
				s.Errorf("Expected path %s not found", expectedFile)
			}
		}
	}

	for _, dump := range response.FileDump {
		if !strings.HasPrefix(dump.Path, param.expectedPrefix) {
			s.Errorf("File %s does not have prefix %s", dump.Path, param.expectedPrefix)
		}

		if dump.CanonicalPath == "" {
			s.Errorf("File %s has an empty cannonical path", dump.Path)
		}
	}
}
