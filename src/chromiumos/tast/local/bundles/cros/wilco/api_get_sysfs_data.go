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

type getSysfsDataParam struct {
	typeField      dtcpb.GetSysfsDataRequest_Type
	epxectedPrefix string
	expectedFiles  []string
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
		Pre:          pre.WilcoDtcSuportdAPI,
		Params: []testing.Param{{
			Name: "hwmon",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_HWMON,
				epxectedPrefix: "/sys/class/hwmon/",
			},
		}, {
			Name: "thermal",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_THERMAL,
				epxectedPrefix: "/sys/class/thermal/",
			},
		}, {
			Name: "firmware_dmi_table",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_FIRMWARE_DMI_TABLES,
				epxectedPrefix: "/sys/firmware/dmi/tables/",
				expectedFiles:  []string{"DMI", "smbios_entry_point"},
			},
		}, {
			Name: "power_supply",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_POWER_SUPPLY,
				epxectedPrefix: "/sys/class/power_supply/",
			},
		}, {
			Name: "backlight",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_BACKLIGHT,
				epxectedPrefix: "/sys/class/backlight/",
			},
		}, {
			Name: "network",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_CLASS_NETWORK,
				epxectedPrefix: "/sys/class/net/",
			},
		}, {
			Name: "cpu",
			Val: getSysfsDataParam{
				typeField:      dtcpb.GetSysfsDataRequest_DEVICES_SYSTEM_CPU,
				epxectedPrefix: "/sys/devices/system/cpu/",
			},
		}},
	})
}

func APIGetSysfsData(ctx context.Context, s *testing.State) {
	param := s.Param().(getSysfsDataParam)

	request := dtcpb.GetSysfsDataRequest{}
	response := dtcpb.GetSysfsDataResponse{}

	request.Type = param.typeField

	if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
		s.Fatal("Unable to get Sysfs files: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(response.FileDump) == 0 {
		s.Fatal("No file dumps available")
	}

	if param.expectedFiles != nil {
		for _, expectedFile := range param.expectedFiles {
			expectedFile = path.Join(param.epxectedPrefix, expectedFile)
			found := false
			for _, dump := range response.FileDump {
				if dump.Path == expectedFile {
					found = true
					break
				}
			}

			if !found {
				s.Log(response.String())
				s.Fatalf("Expected path %s not found", expectedFile)
			}
		}
	}

	for _, dump := range response.FileDump {
		if !strings.HasPrefix(dump.Path, param.epxectedPrefix) {
			s.Log(response.String())
			s.Fatalf("File %s does not have prefix %s", dump.Path, param.epxectedPrefix)
		}
	}
}
