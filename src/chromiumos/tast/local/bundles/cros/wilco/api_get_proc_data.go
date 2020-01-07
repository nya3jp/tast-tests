// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetProcData,
		Desc: "Test sending GetProcData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
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

func APIGetProcData(ctx context.Context, s *testing.State) {
	getProcData := func(ctx context.Context, s *testing.State, typeField dtcpb.GetProcDataRequest_Type, expectedPrefix string, expectedFile string) {
		request := dtcpb.GetProcDataRequest{
			Type: typeField,
		}
		response := dtcpb.GetProcDataResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetProcData", &request, &response); err != nil {
			s.Fatal("Unable to get Proc files: ", err)
		}

		// Error conditions defined by the proto definition.
		if len(response.FileDump) == 0 {
			s.Fatal("No file dumps available")
		}

		for _, dump := range response.FileDump {
			if !strings.HasPrefix(dump.Path, expectedPrefix) {
				s.Errorf("File %s does not have prefix %s", dump.Path, expectedPrefix)
			}

			if dump.CanonicalPath == "" {
				s.Errorf("File %s has an empty cannonical path", dump.Path)
			}

			if len(dump.Contents) == 0 {
				s.Errorf("File %s has no content", dump.Path)
			}
		}

		if expectedFile != "" {
			expectedFile := filepath.Join(expectedPrefix, expectedFile)

			if len(response.FileDump) != 1 {
				s.Errorf("Only expected %s as the result", expectedFile)
			}

			if response.FileDump[0].Path != expectedFile {
				s.Errorf("Expected %s, but got %s", expectedFile, response.FileDump[0].Path)
			}
		}
	}

	for _, param := range []struct {
		// name is the subtest name
		name string
		// typeField is sent as the request type to GetProcData.
		typeField dtcpb.GetProcDataRequest_Type
		// expectedPrefix is a prefix that all returned paths must have.
		expectedPrefix string
		// expectedFile is the expected file relative to expectedPrefix.
		expectedFile string
	}{
		{
			name:           "uptime",
			typeField:      dtcpb.GetProcDataRequest_FILE_UPTIME,
			expectedPrefix: "/proc/",
			expectedFile:   "uptime",
		}, {
			name:           "meminfo",
			typeField:      dtcpb.GetProcDataRequest_FILE_MEMINFO,
			expectedPrefix: "/proc/",
			expectedFile:   "meminfo",
		}, {
			name:           "loadavg",
			typeField:      dtcpb.GetProcDataRequest_FILE_LOADAVG,
			expectedPrefix: "/proc/",
			expectedFile:   "loadavg",
		}, {
			name:           "stat",
			typeField:      dtcpb.GetProcDataRequest_FILE_STAT,
			expectedPrefix: "/proc/",
			expectedFile:   "stat",
		}, {
			name:           "acpi_button",
			typeField:      dtcpb.GetProcDataRequest_DIRECTORY_ACPI_BUTTON,
			expectedPrefix: "/proc/acpi/button/",
		}, {
			name:           "netstat",
			typeField:      dtcpb.GetProcDataRequest_FILE_NET_NETSTAT,
			expectedPrefix: "/proc/net/",
			expectedFile:   "netstat",
		}, {
			name:           "net_dev",
			typeField:      dtcpb.GetProcDataRequest_FILE_NET_DEV,
			expectedPrefix: "/proc/net/",
			expectedFile:   "dev",
		}, {
			name:           "diskstats",
			typeField:      dtcpb.GetProcDataRequest_FILE_DISKSTATS,
			expectedPrefix: "/proc/",
			expectedFile:   "diskstats",
		}, {
			name:           "cpuinfo",
			typeField:      dtcpb.GetProcDataRequest_FILE_CPUINFO,
			expectedPrefix: "/proc/",
			expectedFile:   "cpuinfo",
		}, {
			name:           "vmstat",
			typeField:      dtcpb.GetProcDataRequest_FILE_VMSTAT,
			expectedPrefix: "/proc/",
			expectedFile:   "vmstat",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			getProcData(ctx, s, param.typeField, param.expectedPrefix, param.expectedFile)
		})
	}
}
