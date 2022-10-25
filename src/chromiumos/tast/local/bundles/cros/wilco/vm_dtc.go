// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VMDTC,
		Desc: "Starts an instance of the Wilco DTC VM and tests the DTC (Diagnostics and Telemetry Controller) binaries using partner provided utilities",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"}, // b/217770420
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func VMDTC(ctx context.Context, s *testing.State) {
	const (
		libraryPath = "LD_LIBRARY_PATH=/opt/dtc/lib/ddv/"
		testParams  = `{
			"Cmd": "RunTest",
			"TestList": [
			    {
			        "Test": "Battery",
			        "Args": {
			            "low_mah": 1000,
			            "high_mah": 10000
			        }
			    },
			    {
			        "Test": "MemoryTest",
			        "Args": {
			            "size_kilobytes": 32
			        }
			    }
			]
		}`
	)

	// Shorten the total context by 15 seconds to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	config := wilco.DefaultVMConfig()
	config.TestDBusConfig = true
	if err := wilco.StartVM(ctx, config); err != nil {
		s.Fatal("Unable to Start Wilco DTC VM: ", err)
	}
	defer wilco.StopVM(cleanupCtx)

	if err := wilco.StartSupportd(ctx); err != nil {
		s.Fatal("Unable to start the Wilco DTC Support Daemon: ", err)
	}
	defer wilco.StopSupportd(cleanupCtx)

	// Wait for ddv dbus service to be up and running before starting
	// test.
	if err := wilco.WaitForDDVDBus(ctx); err != nil {
		s.Fatal("DDV dbus service not available: ", err)
	}

	// test-ddv -g (generate summary report)
	// test-ddv -s (examine single and two summary alert)
	// test-ddv -r (examine runtime summary alert)
	for _, param := range []string{"-g", "-s", "-r"} {
		s.Log("Running test-ddv ", param)
		cmd := vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, libraryPath, "test-ddv", param)
		if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Error running test-ddv %v: %v", param, err)
		} else if !strings.Contains(string(out), "success") {
			s.Errorf("test-ddv %v output does not contain `success`: %s", param, out)
		}
	}

	// test-ddtm -cmd calls wilco_dtc_supportd outside of the VM to run a
	// diagnostic test.
	s.Log("Running test-ddtm -cmd")
	cmd := vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, libraryPath, "test-ddtm", "-cmd", testParams)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Error("Error running test-ddtm -cmd: ", err)
	} else if !strings.Contains(string(out), "Finish DDTM test") {
		s.Errorf("test-ddtm -cmd output does not contain `Finish DDTM test`: %s", out)
	} else if strings.Contains(string(out), "fail") {
		s.Errorf("test-ddtm -cmd output contains `fail` %s", out)
	}
}
