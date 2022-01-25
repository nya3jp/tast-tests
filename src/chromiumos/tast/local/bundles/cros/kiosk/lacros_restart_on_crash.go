// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosproc"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosRestartOnCrash,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Kiosk Lacros properly restarts",
		Contacts: []string{
			"zubeil@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
	})
}

func LacrosRestartOnCrash(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.ExtraChromeOptions(chromeOptions),
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	testing.ContextLog(ctx, "Waiting for splash screen to be  gone")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return kioskmode.IsKioskAppStarted(ctx)
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Splash screen is not gone: ", err)
	}

	// Start reader for /var/log/messages to check kiosk mode has started.
	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	// Start reader for chrome log to check that lacros was used for kiosk mode.
	chromeReader, err := syslog.NewReader(ctx, syslog.SourcePath(cr.LogFilename()))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer chromeReader.Close()

	// Kill lacros using SIGSEGV signal to simulate a browser crash.
	s.Log("Killing lacros")
	proc, err := lacrosproc.Root(lacrosproc.Rootfs)
	if err != nil {
		s.Fatal("Failed to get lacros proc: ", err)
	}

	if err := proc.SendSignalWithContext(ctx, syscall.SIGSEGV); err != nil {
		s.Fatal("Failed to crash chrome: ", err)
	}

	// Check that the kiosk recovers and lacros is used as browser.
	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		s.Fatal("Failed to start kiosk mode after killing: ", err)
	}

	// Check that lacros was used for kiosk mode.
	const expectedLogMsg = "Using lacros-chrome for web kiosk session."
	s.Log("Waiting for lacros log message")
	if _, err := chromeReader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, expectedLogMsg)
		}); err != nil {
		s.Errorf("Failed to wait for log msg \"%q\": %v", expectedLogMsg, err)
	}
}
