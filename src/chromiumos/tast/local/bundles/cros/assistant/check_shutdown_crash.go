// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
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
		Contacts:     []string{"wutao@chromium.org", "xiaohuic@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

func checksumOfFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hasCrashReporterInLog(ctx context.Context, file string) (bool, error) {
	logFile, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer logFile.Close()

	const crashReporter = "chrome_crashpad_handler"
	logScanner := bufio.NewScanner(logFile)
	for logScanner.Scan() {
		// Read line by line and check if it contains the string.
		if strings.Contains(logScanner.Text(), crashReporter) {
			return true, nil
		}
	}
	return false, nil
}

func CheckShutdownCrash(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	// Create test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// Check `chrome.PREVIOUS` log.
	const chromePreviousLog = "/var/log/chrome/chrome.PREVIOUS"
	// Read and hash the current `chrome.PREVIOUS` log as a reference, to make
	// sure we parse an updated `chrome.PREVIOUS` log after the UI restarts.
	s.Log(ctx, "Read and hash `chrome.PREVIOUS` log")
	checksum, err := checksumOfFile(chromePreviousLog)
	if err != nil {
		s.Fatal("Failed to open `chrome.PREVIOUS`: ", err)
	}

	// Restart ui job to get to the logout state.
	s.Log(ctx, "Restart ui to simulate shutdown")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Wait for `chrome.PREVIOUS` log to be updated.
	s.Log(ctx, "Wait until `chrome.PREVIOUS` log is updated")
	if err := testing.Poll(ctx, func(c context.Context) error {
		newChecksum, err := checksumOfFile(chromePreviousLog)
		if err != nil || newChecksum == checksum {
			return errors.Wrap(err, "`chrome.PREVIOUS` was not updated")
		}

		s.Log(ctx, "Checking if `chrome.PREVIOUS` log has crash info")
		hasCrash, err := hasCrashReporterInLog(ctx, chromePreviousLog)
		if err != nil {
			return errors.Wrap(err, "could not open file")
		}
		if hasCrash {
			return testing.PollBreak(errors.Wrap(err, "Chrome probably crashed"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Something was wrong when checking the `chrome.PREVIOUS` log: ", err)
	}

	// Login and clean up Assistant
	s.Log(ctx, "Login and clean up Assistant")
	cr, err = chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	// Create test API connection.
	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
		s.Fatal("Failed to clean up Assistant: ", err)
	}
}
