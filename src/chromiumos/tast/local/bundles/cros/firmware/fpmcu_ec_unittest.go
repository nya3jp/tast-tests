// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpmcuEcUnittest,
		Desc: "Flashes a unittest binary to the FPMCU and verifies it passes",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func FpmcuEcUnittest(ctx context.Context, s *testing.State) {
	// Setup a servo host connected to FPMCU.
	cmdServod := testexec.CommandContext(ctx, "servod", "--board=bloonchipper")
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdServod.Args))
	if err := cmdServod.Start(); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmdServod.Args), err)
	}

	// Wait for servod to initialize.
	testing.Sleep(ctx, 8*time.Second)

	// TODO(yichengli): Flash the unittest binary.

	s.Log("Getting FPMCU's UART console")
	cmd := testexec.CommandContext(ctx, "dut-control", "raw_fpmcu_uart_pty")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get FPMCU UART console: ", err)
	}

	consolePath := strings.TrimSpace(strings.Split(string(output), ":")[1])
	s.Log("UART console: ", consolePath)

	shell := fmt.Sprintf("echo runtest > %s", consolePath)
	cmd = testexec.CommandContext(ctx, "sh", "-c", shell)
	err = cmd.Run()
	if err != nil {
		s.Fatal("Failed to execute runtest from UART: ", err)
	}

	file, openErr := os.Open(consolePath)
	if openErr != nil {
		s.Fatal("Failed to open UART for result: ", openErr)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		s.Log(line)
		if strings.Contains(line, "Pass!") {
			return
		}
		if strings.Contains(line, "Fail!") {
			s.Fatal("Unittest failed on device")
		}
	}
	// TODO(yichengli): Set a timeout
}
