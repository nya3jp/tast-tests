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
	"regexp"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type imageType int

const (
	// imageTypeRW should be the default in testMetadata.
	imageTypeRW      imageType = iota
	imageTypeRO      imageType = iota
	maxFlashAttempts           = 2
)

var dataAccessViolation8020000Regex = regexp.MustCompile(
	"Data access violation, mfar = 8020000")
var dataAccessViolation8040000Regex = regexp.MustCompile(
	"Data access violation, mfar = 8040000")
var dataAccessViolation20000000Regex = regexp.MustCompile(
	"Data access violation, mfar = 20000000")

type testMetadata struct {
	name           string
	image          imageType
	hwWriteProtect bool
	// Args to append to "runtest" command.
	testArgs []string
	// Possible regexes that should terminate the test.
	finishRegexes []*regexp.Regexp
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
		// Flashing the FPMCU can take 2 minutes, so allow more time.
		Timeout: 4 * time.Minute,
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
			Val:       testMetadata{name: "bloonchipper/test-flash_physical.bin", image: imageTypeRO},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_flash_write_protect",
			Val:       testMetadata{name: "bloonchipper/test-flash_write_protect.bin", image: imageTypeRO, hwWriteProtect: true},
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
			Name:      "bloonchipper_mpu_ro",
			Val:       testMetadata{name: "bloonchipper/test-mpu.bin", image: imageTypeRO, finishRegexes: []*regexp.Regexp{dataAccessViolation20000000Regex}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_mpu_rw",
			Val:       testMetadata{name: "bloonchipper/test-mpu.bin", finishRegexes: []*regexp.Regexp{dataAccessViolation20000000Regex}},
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
			Name:      "bloonchipper_rollback_region0",
			Val:       testMetadata{name: "bloonchipper/test-rollback.bin", testArgs: []string{"region0"}, finishRegexes: []*regexp.Regexp{dataAccessViolation8020000Regex}},
		}, {
			ExtraAttr: []string{"fingerprint-mcu_dragonclaw"},
			Name:      "bloonchipper_rollback_region1",
			Val:       testMetadata{name: "bloonchipper/test-rollback.bin", testArgs: []string{"region1"}, finishRegexes: []*regexp.Regexp{dataAccessViolation8040000Regex}},
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
func getFpmcuBoardName(testName string) string {
	return strings.Split(testName, "/")[0]
}

// setupServo sets up a servo host connected to FPMCU.
func setupServo(ctx context.Context, testName string) (*testexec.Cmd, error) {
	cmdServod := testexec.CommandContext(ctx, "servod", "--board="+getFpmcuBoardName(testName))
	stdout, err := cmdServod.StdoutPipe()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot watch stdout for %q", shutil.EscapeSlice(cmdServod.Args))
	}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmdServod.Args))
	if err := cmdServod.Start(); err != nil {
		return nil, errors.Wrapf(err, "%q failed", shutil.EscapeSlice(cmdServod.Args))
	}
	// Wait for servod to initialize.
	sc := bufio.NewScanner(stdout)
	for {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return nil, errors.Wrap(err, "error while scanning servo output")
			}
			continue
		}
		t := sc.Text()
		if strings.Contains(t, "INFO - Listening on localhost port") {
			break
		}
	}
	return cmdServod, nil
}

// extractBinaryToFlash extracts the chosen binary to flash to the FPMCU.
func extractBinaryToFlash(ctx context.Context, binaryToFlash, tempDir, tarballPath string) error {
	// The specific binary is the first string in "Val" in the test params.
	cmdUntar := testexec.CommandContext(ctx, "tar", "-C", tempDir, "-xvjf", tarballPath, binaryToFlash)
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmdUntar.Args))
	if err := cmdUntar.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(cmdUntar.Args))
	}
	return nil
}

// flashUnittestBinary flashes the unittest binary to the FPMCU connected to the target.
func flashUnittestBinary(ctx context.Context, testName, dataPath string) error {
	// The default working directory is /root, which isn't writable.
	dir, err := ioutil.TempDir("", "tast.firmware.FpmcuUnittest.")
	if err != nil {
		return errors.Wrap(err, "failed to create temp directory")
	}
	defer os.RemoveAll(dir)

	binaryToFlash := testName
	if err := extractBinaryToFlash(ctx, binaryToFlash, dir, dataPath); err != nil {
		return err
	}
	imageOption := fmt.Sprintf("--image=%s", filepath.Join(dir, binaryToFlash))
	// Flashing the chip may fail due to hardware reasons. Allow |maxFlashAttempts|.
	attempt := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		attempt++
		if attempt > maxFlashAttempts {
			return testing.PollBreak(errors.Errorf("failed to flash FPMCU after %d attempts", maxFlashAttempts))
		}
		cmdFlash := testexec.CommandContext(ctx, "flash_ec", "--board="+getFpmcuBoardName(testName), imageOption)
		testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmdFlash.Args))
		err = cmdFlash.Run(testexec.DumpLogOnError)
		if err != nil {
			testing.ContextLogf(ctx, "Flashing failed on attempt %d", attempt)
		}
		return err
	}, &testing.PollOptions{Interval: 5 * time.Second, Timeout: 4 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to flash unittest binary")
	}
	return nil
}

// fpmcuPower toggles FPMCU power on or off.
func fpmcuPower(ctx context.Context, on bool) error {
	powerArg := "pp3300"
	if !on {
		powerArg = "off"
	}
	cmd := testexec.CommandContext(ctx, "dut-control", fmt.Sprintf("fpmcu_pp3300:%s", powerArg))
	if err := cmd.Run(); err != nil {
		testing.ContextLogf(ctx, "Failed to toggle power to %q: %v", powerArg, err)
		return err
	}
	return nil
}

// rebootFpmcu turns off and then turns on power for FPMCU.
func rebootFpmcu(ctx context.Context) error {
	if err := fpmcuPower(ctx, false); err != nil {
		return err
	}
	testing.Sleep(ctx, time.Second)
	if err := fpmcuPower(ctx, true); err != nil {
		return err
	}
	return nil
}

// hwWriteProtect toggles hardware write protect
func hwWriteProtect(ctx context.Context, on bool) error {
	// fw_wp_en allows servo to control the hardware write protect.
	err := testexec.CommandContext(ctx, "dut-control", "fw_wp_en:on").Run()
	if err != nil {
		return errors.Wrap(err, "failed to enable control of write protect")
	}
	wpArg := "force_off"
	if on {
		wpArg = "force_on"
	}
	err = testexec.CommandContext(ctx, "dut-control", fmt.Sprintf("fw_wp_state:%s", wpArg)).Run()
	if err != nil {
		return errors.Wrapf(err, "failed to toggle write protect to %q", wpArg)
	}
	return nil
}

// getFpmcuConsolePath returns FPMCU UART console's PTY.
func getFpmcuConsolePath(ctx context.Context) (string, error) {
	testing.ContextLog(ctx, "Getting FPMCU's UART console")
	cmd := testexec.CommandContext(ctx, "dut-control", "raw_fpmcu_console_uart_pty")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to get FPMCU UART console: ")
	}
	// Example output: raw_fpmcu_console_uart_pty:/dev/pts/8
	return strings.TrimSpace(strings.Split(string(output), ":")[1]), nil
}

// scanConsole scans the FPMCU console for any line in finishLines.
// On timeout, it closes the console to kill the scanner.
func scanConsole(ctx context.Context, scanner *bufio.Scanner, console *os.File, finishLines []string, timeout time.Duration) error {
	// scanner.Scan() below might block. Release bufio.Scanner by closing
	// "console" on timeout.
	scanCtx, scanCancel := context.WithTimeout(ctx, timeout)
	defer scanCancel()
	finished := false
	go func() {
		defer func() {
			if !finished {
				testing.ContextLog(ctx, "Timed out scanning FPMCU console, killing the console to release scanner")
				console.Close()
				console = nil
			}
		}()
		// Blocks until deadline is passed.
		<-scanCtx.Done()
	}()

	for {
		if !scanner.Scan() {
			finished = true
			if err := scanner.Err(); err != nil {
				return errors.Wrap(scanner.Err(), "error while scanning FPMCU console")
			}
			return errors.New("EOF while scanning FPMCU console")
		}
		t := scanner.Text()
		for _, line := range finishLines {
			if strings.Contains(t, line) {
				finished = true
				return nil
			}
		}
	}
}

func FpmcuUnittest(ctx context.Context, s *testing.State) {
	metadata := s.Param().(testMetadata)

	cmdServod, err := setupServo(ctx, metadata.name)
	if err != nil {
		s.Fatal("Failed to start servod: ", err)
	}
	defer cmdServod.Wait(testexec.DumpLogOnError)
	defer cmdServod.Signal(syscall.SIGINT)

	// Reboot the FPMCU for a clean state.
	if err := rebootFpmcu(ctx); err != nil {
		s.Fatal("Failed to reboot FPMCU: ", err)
	}

	consolePath, err := getFpmcuConsolePath(ctx)
	if err != nil {
		s.Fatal("Failed to get FPMCU console path: ", err)
	}

	console, err := os.OpenFile(consolePath, os.O_APPEND|os.O_RDWR|syscall.O_NOCTTY, 0644)
	if err != nil {
		s.Fatal("Failed to open FPMCU console: ", err)
	}

	// Schedule cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Tearing down")
		// console.Close() may have been called to kill scanner.
		if console != nil {
			console.Close()
		}
		if err := rebootFpmcu(ctx); err != nil {
			s.Error("Failed to reboot FPMCU in cleanup: ", err)
		}
	}(cleanupCtx)

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

	if err := flashUnittestBinary(ctx, metadata.name, s.DataPath("fpmcu_unittests.tar.bz2")); err != nil {
		s.Fatal("Failed to flash unittest binary: ", err)
	}

	s.Log("Waiting for FPMCU to reboot after flashing")
	// Two seconds should be more than enough for the chip to boot.
	testing.Sleep(ctx, 2*time.Second)
	s.Log("Checking that FPMCU rebooted to RW")
	if _, err = console.Write([]byte("sysinfo\n")); err != nil {
		s.Error("Failed to write command to FPMCU: ", err)
	}
	if err := scanConsole(ctx, scanner, console, []string{"Image: RW", "Copy:   RW"}, 10*time.Second); err != nil {
		s.Fatal("Failed to check FPMCU rebooted to RW: ", err)
	}
	s.Log("FPMCU rebooted to RW")

	if err := hwWriteProtect(ctx, s.Param().(testMetadata).hwWriteProtect); err != nil {
		s.Fatal("Failed to initialize HW write protect: ", err)
	}

	if metadata.image == imageTypeRO {
		s.Log("Rebooting FPMCU to RO")
		// Sometimes the FPMCU console can be read but cannot be written.
		// In that case we will not have the error here, and we have no
		// way to reboot FPMCU to RO.
		if _, err = console.Write([]byte("reboot ro\n")); err != nil {
			s.Fatal("Failed to switch FPMCU to RO: ", err)
		}
		// Two seconds should be more than enough for the chip to boot.
		testing.Sleep(ctx, 2*time.Second)
		s.Log("Checking that FPMCU rebooted to RO")
		if _, err = console.Write([]byte("sysinfo\n")); err != nil {
			s.Error("Failed to write command to FPMCU: ", err)
		}
		if err := scanConsole(ctx, scanner, console, []string{"Image: RO", "Copy:   RO"}, 10*time.Second); err != nil {
			s.Fatal("Failed to check FPMCU rebooted to RO: ", err)
		}
		s.Log("FPMCU rebooted to RO")
	}

	s.Log("Running FPMCU unittest through UART console: ", consolePath)
	cmd := fmt.Sprintf("runtest %s\n", strings.Join(metadata.testArgs, " "))
	if _, err = console.Write([]byte(cmd)); err != nil {
		s.Fatal("Failed to execute runtest from FPMCU console: ", err)
	}

	if len(metadata.finishRegexes) == 0 {
		metadata.finishRegexes = []*regexp.Regexp{regexp.MustCompile("Pass!")}
	}

	// scanner.Scan() below might block. Release bufio.Scanner by closing
	// "console" on timeout.
	scanCtx, scanCancel := context.WithTimeout(ctx, 30*time.Second)
	defer scanCancel()
	go func() {
		defer func() {
			console.Close()
			console = nil
		}()
		// Blocks until deadline is passed.
		<-scanCtx.Done()
	}()

	finished := false
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := logWriter.WriteString(line + "\n"); err != nil {
			s.Error("Failed to write line to ", consoleOutputPath)
		}
		if strings.Contains(line, "Fail!") {
			s.Fatal("Unittest failed on device")
		}
		if !finished {
			for _, re := range metadata.finishRegexes {
				if re.MatchString(line) {
					finished = true
					break
				}
			}
			// Continue scanning until EOF or timeout to see if we
			// have a "Fail!" coming.
		}
	}
	if !finished {
		s.Fatal("Failed to scan unittest result")
	}
}
