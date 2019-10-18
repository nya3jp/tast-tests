// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wilco/common"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

type getSysfsDataParam struct {
	typeField     dtcpb.GetSysfsDataRequest_Type
	expectedFiles []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetSysfsData,
		Desc: "Test GetSysfsData in WilcoDtcSupportd",
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
				typeField:     dtcpb.GetSysfsDataRequest_CLASS_HWMON,
				expectedFiles: []string{"/sys/class/hwmon/hwmon0/name", "/sys/class/hwmon/hwmon1/name"},
			},
		}, {
			Name: "thermal",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_CLASS_THERMAL,
				expectedFiles: []string{"/sys/class/thermal/thermal_zone3/temp", "/sys/class/thermal/cooling_device4/type"},
			},
		}, {
			Name: "firmware_dmi_table",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_FIRMWARE_DMI_TABLES,
				expectedFiles: []string{"/sys/firmware/dmi/tables/DMI", "/sys/firmware/dmi/tables/smbios_entry_point"},
			},
		}, {
			Name: "power_supply",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_CLASS_POWER_SUPPLY,
				expectedFiles: []string{"/sys/class/power_supply/AC/type", "/sys/class/power_supply/BAT0/type", "/sys/class/power_supply/wilco-charger/type"},
			},
		}, {
			Name: "backlight",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_CLASS_BACKLIGHT,
				expectedFiles: []string{"/sys/class/backlight/intel_backlight/actual_brightness"},
			},
		}, {
			Name: "network",
			Val: getSysfsDataParam{
				typeField: dtcpb.GetSysfsDataRequest_CLASS_NETWORK,
				// Network interface names can change, making test flaky. lo is not exported.
				expectedFiles: []string{},
			},
		}, {
			Name: "cpu",
			Val: getSysfsDataParam{
				typeField:     dtcpb.GetSysfsDataRequest_DEVICES_SYSTEM_CPU,
				expectedFiles: []string{"/sys/devices/system/cpu/cpu0/uevent", "/sys/devices/system/cpu/cpu1/uevent"},
			},
		}},
	})
}

func pathInDumps(p string, dumps []*dtcpb.FileDump) bool {
	for _, e := range dumps {
		if e.Path == p {
			return true
		}
	}
	return false
}

func APIGetSysfsData(ctx context.Context, s *testing.State) {
	cleanupCtx, ctx, err := common.SetupSuportdForAPITest(ctx, s)
	defer common.TeardownSuportdForAPITest(cleanupCtx, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	sdMsg := dtcpb.GetSysfsDataRequest{}
	sdRes := dtcpb.GetSysfsDataResponse{}

	sdMsg.Type = s.Param().(getSysfsDataParam).typeField

	if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &sdMsg, &sdRes); err != nil {
		s.Fatal("Unable to get Sysfs files: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(sdRes.FileDump) == 0 {
		s.Fatal("No file dumps available")
	}

	for _, expectedFile := range s.Param().(getSysfsDataParam).expectedFiles {
		if !pathInDumps(expectedFile, sdRes.FileDump) {
			s.Log(sdRes.String())
			s.Fatalf("Expected path %s not found", expectedFile)
		}
	}
}
