// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var (
	enableWPPrompt  = "Prompt for hardware WP able"
	disableWPPrompt = "Prompt for hardware WP disable"
	continuePrompt  = "Press enter to continue"

	subtestResultPrefix = "<+>"
	subtestPass         = "Pass"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlashromTester,
		Desc:         "Tast wrapper that runs flashrom_tester",
		Contacts:     []string{"nartemiev@google.com", "cros-flashrom-team@google.com"},
		Attr:         []string{}, // TODO(b/239126062): Move to custom suite/schedule.
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      60 * time.Minute,
		Params: []testing.Param{
			{
				Val:     "--flashrom_binary=/usr/sbin/flashrom",
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "libflashrom",
				Val:     "--libflashrom",
				Fixture: fixture.NormalMode,
			},
		},
	})
}

func FlashromTester(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Create a backup of AP flash to restore.
	// flashrom_tester normally does this, but can't if it times out.
	backupTmpFileStdout, err := h.DUT.Conn().CommandContext(ctx, "mktemp", "-t", "FlashromTesterAPFW.XXXXXXXXXX").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to create a temp file: ", err)
	}
	backupTmpFile := strings.TrimSpace(string(backupTmpFileStdout))
	defer func() {
		h.DUT.Conn().CommandContext(ctx, "rm", backupTmpFile).Output(ssh.DumpLogOnError)
	}()
	if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "host", "-r", backupTmpFile).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create a AP firmware backup: ", err)
	}

	// Reserve time for the backup to be restored if flashrom_tester times out
	// This time is also used by the faft fixture.
	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Reset hardware write protect")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			s.Error("Failed to reset hardware write protect: ", err)
		}

		s.Log("Reset software write protect")
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "host", "--wp-disable", "--wp-range=0,0").Output(ssh.DumpLogOnError); err != nil {
			s.Error("Failed to reset software write protect: ", err)
		}

		// TODO only do this when flashrom_tester timed out?
		// TODO is there a more correct way, eg using futility. Problem is we want to restore hwid and friends as well as firmware.
		s.Log("Restore AP firmware")
		if _, err := h.DUT.Conn().CommandContext(ctx, "flashrom", "-p", "host", "--noverify", "-w", backupTmpFile).Output(ssh.DumpLogOnError); err != nil {
			s.Error("Failed to restore AP firmware backup: ", err)
		}
	}(cleanupContext)

	backendChoiceArg := s.Param().(string)
	cmd := h.DUT.Conn().CommandContext(ctx, "flashrom_tester", "--debug", backendChoiceArg, "host")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("StdinPipe() failed: ", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("StdoutPipe() failed: ", err)
	}
	// duplicate cmd stdout to a log file and a scanner
	stdoutFile, err := os.Create(filepath.Join(s.OutDir(), "flashrom_tester_stdout.txt"))
	if err != nil {
		s.Fatal("os.Open failed: ", err)
	}
	defer func() {
		if err := stdoutFile.Close(); err != nil {
			s.Error("flashrom_tester failed to close stdout: ", err)
		}
	}()
	stdout := io.TeeReader(stdoutPipe, stdoutFile)
	stdoutSc := bufio.NewScanner(stdout)

	stderrFile, err := os.Create(filepath.Join(s.OutDir(), "flashrom_tester_stderr.txt"))
	if err != nil {
		s.Fatal("os.Open failed: ", err)
	}
	defer func() {
		if err := stderrFile.Close(); err != nil {
			s.Error("Failed to close stderr: ", err)
		}
	}()
	cmd.Stderr = stderrFile

	s.Log("Starting flashrom_tester")
	if err := cmd.Start(); err != nil {
		s.Fatal("Start() failed: ", err)
	}

	defer func() {
		if err := cmd.Wait(); err != nil {
			s.Error("flashrom_tester failed: ", err)
		}
	}()

	for stdoutSc.Scan() {
		text := stdoutSc.Text()
		// Find output lines that contain a non-passing subtest result
		// Example subtest results:
		//    <+> Lock_top_quad test: Pass
		//    <+> Lock_bottom_quad test: Fail
		if strings.Contains(text, subtestResultPrefix) && !strings.Contains(text, subtestPass) {
			s.Error(text)
		}

		// Change HWWP when prompted by the tester
		changeWP := false
		targetWPState := servo.FWWPStateOff
		wpStr := "disable"
		if strings.Contains(text, disableWPPrompt) {
			changeWP = true
		} else if strings.Contains(text, enableWPPrompt) {
			changeWP = true
			targetWPState = servo.FWWPStateOn
			wpStr = "enable"
		}
		if changeWP {
			s.Logf("Handling prompt to %s WP", wpStr)

			if err := h.Servo.SetFWWPState(ctx, targetWPState); err != nil {
				s.Fatalf("Failed to %s WP: %v", wpStr, err)
			}
		}
		if strings.Contains(text, continuePrompt) {
			// Write newline because the tester expects a key press
			s.Log("Continuing test")
			if _, err := io.WriteString(stdin, "\n"); err != nil {
				s.Fatal("WriteString() failed: ", err)
			}
		}
	}
	if err := stdoutSc.Err(); err != nil {
		s.Fatal("Reading standard output failed: ", err)
	}
}
