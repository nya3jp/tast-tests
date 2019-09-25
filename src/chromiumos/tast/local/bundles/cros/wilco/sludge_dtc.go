// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	wvm "chromiumos/tast/local/bundles/cros/wilco/vm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SludgeDTC,
		Desc: "Starts an instance of the Wilco DTC VM tests the DTC (Diagnostics and Telemetry Controller) binaries",
		Contacts: []string{
			"tbegin@chromium.org", // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd author
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func SludgeDTC(ctx context.Context, s *testing.State) {
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

	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Expect the VM to start within 5 seconds.
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	config := wvm.DefaultSludgeConfig()
	config.TestDbusConfig = true
	if err := wvm.StartSludge(startCtx, config); err != nil {
		s.Fatal("Unable to Start Sludge VM: ", err)
	}
	defer wvm.StopSludge(cleanupCtx)

	if err := wvm.StartWilcoSupportDaemon(startCtx); err != nil {
		s.Fatal("Unable to Start Sludge VM: ", err)
	}
	defer wvm.StopWilcoSupportDaemon(cleanupCtx)

	// Wait for com.dell.ddv dbus service to be up and running before starting
	// test.
	cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID,
		"gdbus", "wait", "--system", "--timeout", "5", "com.dell.ddv")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("DDV dbus service not available: ", err)
	}

	// test-ddv -g (generate summary report)
	// test-ddv -s (examine single and two summary alert)
	// test-ddv -r (examine runtime summary alert)
	for _, param := range []string{"-g", "-s", "-r"} {
		cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, libraryPath, "test-ddv", param)
		if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Error running test-ddv %v: %v", param, err)
		} else if !strings.Contains(string(out), "success") {
			s.Errorf("test-ddv %v output does not contain `success`: %s", param, out)
		} else {
			s.Logf("test-ddv %v successful", param)
		}
	}

	// test-ddtm -cmd calls wilco_dtc_supportd outside of the VM to run a
	// diagnostic test.
	cmd = vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, libraryPath, "test-ddtm", "-cmd", testParams)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Error("Error running test-ddtm -cmd: ", err)
	} else if !strings.Contains(string(out), "Finish DDTM test") {
		s.Errorf("test-ddtm -cmd output does not contain `Finish DDTM test`: %s", out)
	} else if strings.Contains(string(out), "fail") {
		s.Errorf("test-ddtm -cmd output contains `fail` %s", out)
	} else {
		s.Log("test-ddtm -cmd successful")
	}
}
