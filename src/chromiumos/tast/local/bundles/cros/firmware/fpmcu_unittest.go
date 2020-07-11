// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpmcuUnittest,
		Desc: "Flashes a unittest binary to the FPMCU and verifies it passes",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Data: []string{"fpmcu_unittests.tar.bz2"},
		Params: []testing.Param{{
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_aes",
			Val:       "bloonchipper/test-aes.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_compile_time_macros",
			Val:       "bloonchipper/test-compile_time_macros.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_crc",
			Val:       "bloonchipper/test-crc.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_flash_physical",
			Val:       "bloonchipper/test-flash_physical.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_flash_write_protect",
			Val:       "bloonchipper/test-flash_write_protect.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_fpsensor",
			Val:       "bloonchipper/test-fpsensor.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_mpu",
			Val:       "bloonchipper/test-mpu.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_mutex",
			Val:       "bloonchipper/test-mutex.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_pingpong",
			Val:       "bloonchipper/test-pingpong.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_rollback",
			Val:       "bloonchipper/test-rollback.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_rollback_entropy",
			Val:       "bloonchipper/test-rollback_entropy.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_rsa3",
			Val:       "bloonchipper/test-rsa3.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_rtc",
			Val:       "bloonchipper/test-rtc.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_scratchpad",
			Val:       "bloonchipper/test-scratchpad.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_sha256",
			Val:       "bloonchipper/test-sha256.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_sha256_unrolled",
			Val:       "bloonchipper/test-sha256_unrolled.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_stm32f_rtc",
			Val:       "bloonchipper/test-stm32f_rtc.bin",
		}, {
			ExtraAttr: []string{"group:fingerprint-dragonclaw"},
			Name:      "bloonchipper_utils",
			Val:       "bloonchipper/test-utils.bin",
		}},
	})
}

// getFpmcuBoardName extracts the FPMCU board name from test params.
func getFpmcuBoardName(s *testing.State) string {
	return strings.Split(s.Param().(string), "/")[0]
}

// setupServo sets up a servo host connected to FPMCU.
func setupServo(ctx context.Context, s *testing.State) {
	cmdServod := testexec.CommandContext(ctx, "servod", "--board="+getFpmcuBoardName(s))
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdServod.Args))
	if err := cmdServod.Start(); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmdServod.Args), err)
	}
	// Wait for servod to initialize.
	testing.Sleep(ctx, 8*time.Second)
}

// extractBinaryToFlash decompresses the tarball and returns the chosen binary to flash to the FPMCU.
func extractBinaryToFlash(ctx context.Context, s *testing.State, tarballPath string) string {
	cmdUntar := testexec.CommandContext(ctx, "tar", "-xjf", tarballPath)
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdUntar.Args))
	if err := cmdUntar.Run(); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmdUntar.Args), err)
	}

	// Relative path to the specific binary is "Val" in the test params.
	return s.Param().(string)
}

// flashUnittestBinary flashes the unittest binary to the FPMCU connected to the target.
func flashUnittestBinary(ctx context.Context, s *testing.State) {
	binaryToFlash := extractBinaryToFlash(ctx, s, s.DataPath("fpmcu_unittests.tar.bz2"))
	imageOption := fmt.Sprintf("--image=%s", binaryToFlash)
	cmdFlash := testexec.CommandContext(ctx, "flash_ec", "--board="+getFpmcuBoardName(s), imageOption)
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdFlash.Args))
	if err := cmdFlash.Run(); err != nil {
		s.Fatal("Flashing unittest binary failed: ", err)
	}
	// Wait for FPMCU to reboot.
	testing.Sleep(ctx, 2*time.Second)
}

// getFpmcuConsolePath returns FPMCU UART console's PTY.
func getFpmcuConsolePath(ctx context.Context, s *testing.State) string {
	s.Log("Getting FPMCU's UART console")
	cmd := testexec.CommandContext(ctx, "dut-control", "raw_fpmcu_uart_pty")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get FPMCU UART console: ", err)
	}
	return strings.TrimSpace(strings.Split(string(output), ":")[1])
}

func FpmcuUnittest(ctx context.Context, s *testing.State) {
	setupServo(ctx, s)
	flashUnittestBinary(ctx, s)
	consolePath := getFpmcuConsolePath(ctx, s)

	console, err := os.OpenFile(consolePath, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		s.Fatal("Failed to open FPMCU console: ", err)
	}
	defer console.Close()

	// Start scanning before writing "runtest" to avoid race.
	scanner := bufio.NewScanner(console)
	consoleOutputPath := "console_output_log"
	s.Log("Writing console output to ", consoleOutputPath)
	logFile, err := os.Create(filepath.Join(s.OutDir(), consoleOutputPath))
	if err != nil {
		s.Fatal("Failed to create file for logging output: ", err)
	}
	defer logFile.Close()
	logWriter := bufio.NewWriter(logFile)
	defer logWriter.Flush()

	s.Log("Running FPMCU unittest through UART console: ", consolePath)
	if _, err = console.Write([]byte("runtest\n")); err != nil {
		s.Fatal("Failed to execute runtest from FPMCU console: ", err)
	}

	// Use the test's default timeout.
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := logWriter.WriteString(line + "\n"); err != nil {
			s.Error("Failed to write line to ", consoleOutputPath)
		}
		if strings.Contains(line, "Pass!") {
			return
		}
		if strings.Contains(line, "Fail!") {
			s.Fatal("Unittest failed on device")
		}
	}
}
