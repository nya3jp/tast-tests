// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type imageType int

const (
	// imageTypeRW should be the default in testMetadata.
	imageTypeRW imageType = iota
	imageTypeRO imageType = iota
)

type testMetadata struct {
	name           string
	image          imageType
	togglePower    bool
	hwWriteProtect bool
	// Args to append to "runtest" command.
	testArgs []string
}

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
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_aes",
			Val:       testMetadata{name: "bloonchipper/test-aes.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_compile_time_macros",
			Val:       testMetadata{name: "bloonchipper/test-compile_time_macros.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_crc",
			Val:       testMetadata{name: "bloonchipper/test-crc.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_flash_physical",
			Val:       testMetadata{name: "bloonchipper/test-flash_physical.bin", image: imageTypeRO, togglePower: true},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_flash_write_protect",
			Val:       testMetadata{name: "bloonchipper/test-flash_write_protect.bin", image: imageTypeRO, togglePower: true, hwWriteProtect: true},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_fpsensor_spi_ro",
			Val:       testMetadata{name: "bloonchipper/test-fpsensor.bin", image: imageTypeRO, testArgs: []string{"spi"}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_fpsensor_spi_rw",
			Val:       testMetadata{name: "bloonchipper/test-fpsensor.bin", testArgs: []string{"spi"}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_fpsensor_uart_ro",
			Val:       testMetadata{name: "bloonchipper/test-fpsensor.bin", image: imageTypeRO, testArgs: []string{"uart"}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_fpsensor_uart_rw",
			Val:       testMetadata{name: "bloonchipper/test-fpsensor.bin", testArgs: []string{"uart"}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_mpu",
			Val:       testMetadata{name: "bloonchipper/test-mpu.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_mutex",
			Val:       testMetadata{name: "bloonchipper/test-mutex.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_pingpong",
			Val:       testMetadata{name: "bloonchipper/test-pingpong.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_rollback",
			Val:       testMetadata{name: "bloonchipper/test-rollback.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_rollback_entropy",
			Val:       testMetadata{name: "bloonchipper/test-rollback_entropy.bin", image: imageTypeRO},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_rsa3",
			Val:       testMetadata{name: "bloonchipper/test-rsa3.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_rtc",
			Val:       testMetadata{name: "bloonchipper/test-rtc.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_scratchpad",
			Val:       testMetadata{name: "bloonchipper/test-scratchpad.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_sha256",
			Val:       testMetadata{name: "bloonchipper/test-sha256.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_sha256_unrolled",
			Val:       testMetadata{name: "bloonchipper/test-sha256_unrolled.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_stm32f_rtc",
			Val:       testMetadata{name: "bloonchipper/test-stm32f_rtc.bin"},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_utils",
			Val:       testMetadata{name: "bloonchipper/test-utils.bin"},
		}},
	})
}

// getFpmcuBoardName extracts the FPMCU board name from test params.
func getFpmcuBoardName(s *testing.State) string {
	testName := s.Param().(testMetadata).name
	return strings.Split(testName, "/")[0]
}

// setupServo sets up a servo host connected to FPMCU.
func setupServo(ctx context.Context, s *testing.State) *testexec.Cmd {
	cmdServod := testexec.CommandContext(ctx, "servod", "--board="+getFpmcuBoardName(s))
	stdout, err := cmdServod.StdoutPipe()
	if err != nil {
		s.Fatalf("Cannot watch stdout for %q: %v", shutil.EscapeSlice(cmdServod.Args), err)
	}
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdServod.Args))
	if err := cmdServod.Start(); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmdServod.Args), err)
	}
	// Wait for servod to initialize.
	sc := bufio.NewScanner(stdout)
	for {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				s.Fatal("Error while scanning servo output: ", err)
			}
			continue
		}
		t := sc.Text()
		if strings.Contains(t, "INFO - Listening on localhost port") {
			break
		}
	}
	return cmdServod
}

// extractBinaryToFlash extracts the chosen binary to flash to the FPMCU.
func extractBinaryToFlash(ctx context.Context, s *testing.State, tempDir, tarballPath string) string {
	// The specific binary is the first string in "Val" in the test params.
	cmdUntar := testexec.CommandContext(ctx, "tar", "-C", tempDir, "-xvjf", tarballPath, s.Param().(testMetadata).name)
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdUntar.Args))
	if err := cmdUntar.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmdUntar.Args), err)
	}

	return s.Param().(testMetadata).name
}

// flashUnittestBinary flashes the unittest binary to the FPMCU connected to the target.
func flashUnittestBinary(ctx context.Context, s *testing.State) error {
	// The default working directory is /root, which isn't writable.
	dir, err := ioutil.TempDir("", "tast.firmware.FpmcuUnittest.")
	if err != nil {
		return errors.Wrap(err, "failed to create temp directory")
	}
	defer os.RemoveAll(dir)

	binaryToFlash := extractBinaryToFlash(ctx, s, dir, s.DataPath("fpmcu_unittests.tar.bz2"))
	imageOption := fmt.Sprintf("--image=%s", filepath.Join(dir, binaryToFlash))
	cmdFlash := testexec.CommandContext(ctx, "flash_ec", "--board="+getFpmcuBoardName(s), imageOption)
	s.Logf("Running command: %q", shutil.EscapeSlice(cmdFlash.Args))
	if err := cmdFlash.Run(); err != nil {
		return err
	}
	return nil
}

func fpmcuPower(ctx context.Context, on bool) error {
	powerArg := "pp3300"
	if !on {
		powerArg = "off"
	}
	cmd := testexec.CommandContext(ctx, "dut-control", fmt.Sprintf("fpmcu_pp3300:%s", powerArg))
	err := cmd.Run()
	if err != nil {
		testing.ContextLogf(ctx, "Failed to toggle power to %q: %v", powerArg, err)
	}
	return err
}

// hwWriteProtect toggles hardware write protect
func hwWriteProtect(ctx context.Context, on bool) error {
	wpArg := "on"
	if !on {
		wpArg = "off"
	}
	cmd := testexec.CommandContext(ctx, "dut-control", fmt.Sprintf("fw_wp_en:%s", wpArg))
	err := cmd.Run()
	if err != nil {
		testing.ContextLogf(ctx, "Failed to toggle write protect to %q: %v", wpArg, err)
	}
	return err
}

// getFpmcuConsolePath returns FPMCU UART console's PTY.
func getFpmcuConsolePath(ctx context.Context, s *testing.State) string {
	s.Log("Getting FPMCU's UART console")
	cmd := testexec.CommandContext(ctx, "dut-control", "raw_fpmcu_console_uart_pty")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get FPMCU UART console: ", err)
	}
	// Example output: raw_fpmcu_console_uart_pty:/dev/pts/8
	return strings.TrimSpace(strings.Split(string(output), ":")[1])
}

func FpmcuUnittest(ctx context.Context, s *testing.State) {
	cmdServod := setupServo(ctx, s)
	defer cmdServod.Wait(testexec.DumpLogOnError)
	defer cmdServod.Signal(syscall.SIGINT)

	// Reboot the FPMCU for a clean state.
	if err := fpmcuPower(ctx, false); err != nil {
		s.Fatal("Failed to reset FPMCU: ", err)
	}
	testing.Sleep(ctx, time.Second)
	if err := fpmcuPower(ctx, true); err != nil {
		s.Fatal("Failed to reset FPMCU: ", err)
	}

	consolePath := getFpmcuConsolePath(ctx, s)

	console, err := os.OpenFile(consolePath, os.O_APPEND|os.O_RDWR|syscall.O_NOCTTY, 0644)
	if err != nil {
		s.Fatal("Failed to open FPMCU console: ", err)
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

	if err := flashUnittestBinary(ctx, s); err != nil {
		// Try reboot for cleanup.
		fpmcuPower(ctx, false)
		testing.Sleep(ctx, time.Second)
		fpmcuPower(ctx, true)
		s.Fatal("Flashing unittest binary failed: ", err)
	}

	// Run the test in RO or RW, depending on "Val" in test params.
	imageToBoot := "RW"
	if s.Param().(testMetadata).image == imageTypeRO {
		imageToBoot = "RO"
	}

	// Wait for FPMCU to reboot after flashing
	for {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				s.Fatal("Error while scanning FPMCU console: ", err)
			}
			continue
		}
		t := scanner.Text()
		if strings.Contains(t, fmt.Sprintf("%s%s", "Image: ", imageToBoot)) {
			break
		}
	}

	if imageToBoot == "RO" {
		if _, err = console.Write([]byte("sysjump ro\n")); err != nil {
			s.Fatal("Failed to switch FPMCU to RO: ", err)
		}
	}

	if s.Param().(testMetadata).togglePower {
		// Reboot the FPMCU for a clean state.
		if err := fpmcuPower(ctx, false); err != nil {
			s.Fatal("Failed to reset FPMCU: ", err)
		}
		testing.Sleep(ctx, time.Second)
		if err := fpmcuPower(ctx, true); err != nil {
			s.Fatal("Failed to reset FPMCU: ", err)
		}
	}

	hwWriteProtect(ctx, s.Param().(testMetadata).hwWriteProtect)

	s.Log("Running FPMCU unittest through UART console: ", consolePath)
	cmd := fmt.Sprintf("runtest %s\n", strings.Join(s.Param().(testMetadata).testArgs, " "))
	if _, err = console.Write([]byte(cmd)); err != nil {
		s.Fatal("Failed to execute runtest from FPMCU console: ", err)
	}

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
	if err := scanner.Err(); err != nil {
		s.Error("Error while scanning FPMCU console: ", err)
	}
	s.Fatal("Failed to scan unittest result")
}
