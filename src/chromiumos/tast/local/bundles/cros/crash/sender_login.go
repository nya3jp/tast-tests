// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

const (
	// crashSenderDonePath is the path that crash_sender will touch when it finishes running.
	// Must match kCrashSenderDone in platform2/crash-reporter/crash_sender_paths.h
	crashSenderDonePath = "/run/crash_reporter/crash-sender-done"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SenderLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test to check that crash_sender runs on login",
		Contacts: []string{
			"mutexlox@chromium.org",
			"iby@chromium.org",
			"cros-telemetry@google.com",
		},
		// We only care about crash_sender on internal builds.
		SoftwareDeps: []string{"chrome", "cros_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      chrome.LoginTimeout + time.Minute,
	})
}

func SenderLogin(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	defer func() {
		if err := os.Remove(crashSenderDonePath); err != nil && !os.IsNotExist(err) {
			s.Log("Failed to clean up crashSenderDonePath: ", err)
		}
	}()

	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes(crash.FilterInIgnoreAllCrashes), crash.WithMockConsent()); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

	// Ensure completion file isn't left over from previous test. By default, we
	// don't print out the error here, nor do we fail the test because of
	// errors from os.Remove. In the many cases, the done file won't exist
	// and we'll get an error about trying to remove an non-existent file.
	// (It's fine that the file doesn't exist; that's what we expect.)
	// Instead of checking the error on this line, we just check on the
	// next line that the file doesn't exist. As long as the file doesn't
	// exist after this line, we're OK with it either being deleted here
	// *or* it not having existed in the first place. However, we do save
	// the error object -- if the file still exists, we want to add it into
	// the error message saying that the remove failed.
	removeErr := os.Remove(crashSenderDonePath)
	if _, err := os.Stat(crashSenderDonePath); err == nil {
		s.Fatal(crashSenderDonePath, " still exists. Remove failed with: ", removeErr)
	} else if !errors.Is(err, os.ErrNotExist) {
		s.Fatal("Could not stat ", crashSenderDonePath, ": ", err)
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	if err := testing.Poll(ctx, func(c context.Context) error {
		_, err := os.Stat(crashSenderDonePath)
		return err
	}, nil); err != nil {
		s.Error("crash-sender-done file not found: ", err)
	}
}
