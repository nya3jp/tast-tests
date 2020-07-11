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
		Attr: []string{"group:fingerprint-mcu"},
		Data: []string{"fpmcu_unittests.tar.bz2"},
		Params: []testing.Param{{
			Name:      "bloonchipper_aes",
			Val:       "bloonchipper/test-aes.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_compile_time_macros",
			Val:       "bloonchipper/test-compile_time_macros.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_crc",
			Val:       "bloonchipper/test-crc.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_flash_physical",
			Val:       "bloonchipper/test-flash_physical.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_flash_write_protect",
			Val:       "bloonchipper/test-flash_write_protect.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_fpsensor",
			Val:       "bloonchipper/test-fpsensor.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_mpu",
			Val:       "bloonchipper/test-mpu.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_mutex",
			Val:       "bloonchipper/test-mutex.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_pingpong",
			Val:       "bloonchipper/test-pingpong.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_rollback",
			Val:       "bloonchipper/test-rollback.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_rollback_entropy",
			Val:       "bloonchipper/test-rollback_entropy.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_rsa3",
			Val:       "bloonchipper/test-rsa3.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_rtc",
			Val:       "bloonchipper/test-rtc.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_scratchpad",
			Val:       "bloonchipper/test-scratchpad.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_sha256",
			Val:       "bloonchipper/test-sha256.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_sha256_unrolled",
			Val:       "bloonchipper/test-sha256_unrolled.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_stm32f_rtc",
			Val:       "bloonchipper/test-stm32f_rtc.bin",
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "bloonchipper_utils",
			Val:       "bloonchipper/test-utils.bin",
			ExtraAttr: []string{"informational"},
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
		s.Fatal("Flasing unittest binary failed: ", err)
	}
	// Wait for FPMCU to reboot.
	testing.Sleep(ctx, 2*time.Second)
}

// getFpmcuConsolePath returns FPMCU UART console's file descriptor.
func getFpmcuConsolePath(ctx context.Context, s *testing.State) string {
	s.Log("Getting FPMCU's UART console")
	cmd := testexec.CommandContext(ctx, "dut-control", "raw_fpmcu_uart_pty")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get FPMCU UART console: ", err)
	}
	return strings.TrimSpace(strings.Split(string(output), ":")[1])
}

// runUnittest issues "runtest" command via FPMCU UART console.
func runUnittest(ctx context.Context, s *testing.State, consolePath string) {
	s.Log("Running FPMCU unittest through UART console: ", consolePath)
	shell := fmt.Sprintf("echo runtest > %s", consolePath)
	cmd := testexec.CommandContext(ctx, "sh", "-c", shell)
	err := cmd.Run()
	if err != nil {
		s.Fatal("Failed to execute runtest from UART: ", err)
	}
}

func FpmcuUnittest(ctx context.Context, s *testing.State) {
	setupServo(ctx, s)
	flashUnittestBinary(ctx, s)
	consolePath := getFpmcuConsolePath(ctx, s)
	runUnittest(ctx, s, consolePath)

	console, err := os.Open(consolePath)
	if err != nil {
		s.Fatal("Failed to open UART for result: ", err)
	}
	defer console.Close()
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

	// Use the test's default timeout.
	for scanner.Scan() {
		line := scanner.Text()
		logWriter.WriteString(line + "\n")
		if strings.Contains(line, "Pass!") {
			return
		}
		if strings.Contains(line, "Fail!") {
			s.Fatal("Unittest failed on device")
		}
	}
}
