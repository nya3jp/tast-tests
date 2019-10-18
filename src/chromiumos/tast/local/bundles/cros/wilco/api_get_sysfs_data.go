// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/wilco/common"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

type getSysfsDataParam struct {
	typeField     dtcpb.GetSysfsDataRequest_Type
	prefix        string
	expectedFiles []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetSysfsData,
		Desc: "Test sending GetSysfsData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Params: []testing.Param{{
			Name: "hwmon",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_HWMON,
				prefix:    "/sys/class/hwmon/",
			},
		}, {
			Name: "thermal",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_THERMAL,
				prefix:    "/sys/class/thermal/",
			},
		}, {
			Name: "firmware_dmi_table",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_FIRMWARE_DMI_TABLES,
				prefix:        "/sys/firmware/dmi/tables/",
				expectedFiles: []string{"/sys/firmware/dmi/tables/DMI", "/sys/firmware/dmi/tables/smbios_entry_point"},
			},
		}, {
			Name: "power_supply",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_POWER_SUPPLY,
				prefix:    "/sys/class/power_supply/",
			},
		}, {
			Name: "backlight",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_BACKLIGHT,
				prefix:    "/sys/class/backlight/",
			},
		}, {
			Name: "network",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_NETWORK,
				prefix:    "/sys/class/net/",
			},
		}, {
			Name: "cpu",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_DEVICES_SYSTEM_CPU,
				prefix:    "/sys/devices/system/cpu/",
			},
		}},
	})
}

func APIGetSysfsData(ctx context.Context, s *testing.State) {
	res, err := common.SetUpSupportdForAPITest(ctx, s)
	ctx = res.TestContext
	defer common.TeardownSupportdForAPITest(res.CleanupContext, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	request := dtcpb.GetSysfsDataRequest{}
	response := dtcpb.GetSysfsDataResponse{}

	request.Type = s.Param().(getSysfsDataParam).typeField

	if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
		s.Fatal("Unable to get Sysfs files: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(response.FileDump) == 0 {
		s.Fatal("No file dumps available")
	}

	pathInDumps := func(path string, dumps []*dtcpb.FileDump) bool {
		for _, dump := range dumps {
			if dump.Path == path {
				return true
			}
		}
		return false
	}

	if s.Param().(getSysfsDataParam).expectedFiles != nil {
		for _, expectedFile := range s.Param().(getSysfsDataParam).expectedFiles {
			if !pathInDumps(expectedFile, response.FileDump) {
				s.Log(response.String())
				s.Fatalf("Expected path %s not found", expectedFile)
			}
		}
	}

	prefixInDumps := func(path string, dumps []*dtcpb.FileDump) bool {
		for _, dump := range dumps {
			if !strings.HasPrefix(dump.Path, path) {
				return false
			}
		}
		return len(dumps) > 0
	}

	if !prefixInDumps(s.Param().(getSysfsDataParam).prefix, response.FileDump) {
		s.Log(response.String())
		s.Fatalf("Prefix %s not root of all dumps", s.Param().(getSysfsDataParam).prefix)
	}
}
