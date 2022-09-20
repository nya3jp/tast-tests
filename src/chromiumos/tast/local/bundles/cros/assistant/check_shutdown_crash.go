// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckShutdownCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check if there was a shutdown crash when Assistant is enabled",
		Contacts:     []string{"wutao@google.com", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          assistant.VerboseLoggingEnabled(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

func hasCrashReporterInLog(ctx context.Context, file string) (bool, error) {
	logFile, err := os.Open(file)
	if err != nil {
		return false, err
	}

	const crashReporter = "chrome_crashpad_handler"
	var hasCrash = false
	logScanner := bufio.NewScanner(logFile)
	logScanner.Split(bufio.ScanLines)
	for logScanner.Scan() {
		// Read line by line and check if it contains the string.
		hasCrash = hasCrash || strings.Contains(logScanner.Text(), crashReporter)
	}
	defer func() {
		logFile.Close()
	}()
	return hasCrash, nil
}

func CheckShutdownCrash(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Create test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// Restart ui job to the logout state.
	testing.ContextLog(ctx, "Restart ui to simulate shutdown")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Wait for chrome.PREVIOUS log to be updated.
	testing.ContextLog(ctx, "Wait 3 seconds to check chrome.PREVIOUS log")
	if err := testing.Poll(ctx, func(c context.Context) error {
		// Check chrome.PREVIOUS log.
		const chromePreviousLog = "/var/log/chrome/chrome.PREVIOUS"
		hasCrash, err := hasCrashReporterInLog(ctx, chromePreviousLog)
		if err != nil {
			return errors.Wrap(err, "could not get crash info")
		}
		if hasCrash {
			return errors.Wrap(err, "chrome crashed")
		}
		// Crash was not found.
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		s.Error("Please check the chrome.PREVIOUS log: ", err)
	}
}
