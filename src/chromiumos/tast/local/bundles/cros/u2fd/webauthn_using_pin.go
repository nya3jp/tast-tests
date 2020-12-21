// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebauthnUsingPIN,
		Desc: "Checks that WebAuthn MakeCredential using PIN succeeds",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"martinkr@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// setUpUserPIN sets up a test user with a specific PIN.
func setUpUserPIN(ctx context.Context, s *testing.State, PIN string) {
	const (
		username   = "testuser@gmail.com"
		password   = "good"
		gaiaID     = "1234"
		autosubmit = true
	)

	var user string
	// Enable device event log in Chrome logs for validation.
	cr, err := chrome.New(ctx, chrome.Auth(username, password, gaiaID), chrome.ExtraArgs("--vmodule=device_event_log*=1"))
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)

	user = cr.User()
	if mounted, err := cryptohome.IsMounted(ctx, user); err != nil {
		s.Fatalf("Failed to check mounted vault for %q: %v", user, err)
	} else if !mounted {
		s.Fatalf("No mounted vault for %q", user)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Set up PIN through a connection to the Settings page.
	if err := ossettings.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	if err := ossettings.EnablePINUnlock(ctx, settingsConn, password, PIN, autosubmit); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}

}

// keyPress presses a key.
func keyPress(ctx context.Context, s *testing.State, keyboard *input.KeyboardEventWriter, keys string) {
	if err := keyboard.Accel(ctx, keys); err != nil {
		s.Fatal("Failed to press ", keys, ": ", err)
	}
	testing.Sleep(ctx, 500*time.Millisecond)
}

func WebauthnUsingPIN(ctx context.Context, s *testing.State) {
	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("Test failed: ", err)
	}

	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	const PIN = "123456"
	setUpUserPIN(ctx, s, PIN)

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Could not get Chrome log reader: ", err)
	}
	defer logReader.Close()

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Open test website in a new tab.
	keyPress(ctx, s, keyboard, "Ctrl+T")

	if err := keyboard.Type(ctx, "https://webauthn.io\n"); err != nil {
		s.Fatal("Failed to type address to test website: ", err)
	}

	// Waiting for test website to load.
	testing.Sleep(ctx, 10*time.Second)

	// Perform MakeCredential on the test website.

	keyPress(ctx, s, keyboard, "Tab")
	if err := keyboard.Type(ctx, "webauthn-tast-test\n"); err != nil {
		s.Fatal("Failed to type username on test website: ", err)
	}

	// Choose platform authenticator
	for _, keys := range []string{"Tab", "Tab", "Enter", "Down", "Down", "Enter"} {
		keyPress(ctx, s, keyboard, keys)
	}

	// Press "Register" button
	for _, keys := range []string{"Tab", "Enter"} {
		keyPress(ctx, s, keyboard, keys)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	// Wait for MakeCredential to finish.
	testing.Sleep(ctx, 3*time.Second)

	assertMakeCredentialSuccess(s, logReader)

	// Perform GetAssertion on the test website.

	// Press "Login" button
	for _, keys := range []string{"Tab", "Enter"} {
		keyPress(ctx, s, keyboard, keys)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	// Wait for GetAssertion to finish.
	testing.Sleep(ctx, 3*time.Second)

	assertGetAssertionSuccess(s, logReader)
}

// assertMakeCredentialSuccess asserts MakeCredential succeeded by looking at Chrome log.
func assertMakeCredentialSuccess(s *testing.State, logReader *syslog.ChromeReader) {
	const makeCredentialSuccessLine = "Make credential status: 1"

	for {
		entry, err := logReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			s.Fatal("Error reading Chrome logs: ", err)
		}
		if strings.Contains(entry.Content, makeCredentialSuccessLine) {
			return
		}
		continue
	}
	s.Fatal("MakeCredential did not succeed")
}

// assertGetAssertionSuccess asserts GetAssertion succeeded by looking at Chrome log.
func assertGetAssertionSuccess(s *testing.State, logReader *syslog.ChromeReader) {
	const getAssertionSuccessLine = "GetAssertion status: 1"

	for {
		entry, err := logReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			s.Fatal("Error reading Chrome logs: ", err)
		}
		if strings.Contains(entry.Content, getAssertionSuccessLine) {
			return
		}
		continue
	}
	s.Fatal("GetAssertion did not succeed")
}
