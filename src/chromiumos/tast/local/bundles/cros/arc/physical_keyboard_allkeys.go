// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardAllkeys,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check any key doesn't crash Android",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func physicalKeyboardAllKeycodesTypingTest(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, d *ui.Device, kb *input.KeyboardEventWriter, s *testing.State) {
	const (
		activityName = ".MainActivity"
		pkg          = "org.chromium.arc.testapp.keyboard"
		fieldID      = "org.chromium.arc.testapp.keyboard:id/text"
	)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", activityName, err)
	}

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity before typing:")
	}
	defer act.Stop(ctx, tconn)

	focusField := func() error {
		field := d.Object(ui.ID(fieldID))
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
		if err != nil {
			return errors.Wrap(err, "failed to get the window info")
		}
		if !info.IsVisible || !info.HasFocus || !info.IsActive {
			return errors.New("the app window is not focused")
		}
		if err := field.WaitForExists(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the field")
		}
		if err := d.Object(ui.ID(fieldID), ui.Focused(true)).Exists(ctx); err != nil {
			if err := field.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the field")
			}
		}
		if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to focus the field")
		}
		return nil
	}

	// The channel to make the logcat monitor stop monitoring.
	done := make(chan bool, 1)
	// The channel to make the logcat monitor report any failure in logcat while monitoring.
	result := make(chan error)
	// This goroutine monitors logcat output to find any mojo connection errors of ArcInputMethodService.
	go func(done chan bool) {
		exp := regexp.MustCompile(`ArcInputMethod: Mojo connection error`)

		notFound := make(chan bool, 1)
		isFinished := func() bool {
			select {
			case <-done:
				notFound <- true
				return true
			default:
				return false
			}
		}

		if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp), isFinished); err != nil {
			result <- errors.Wrap(err, "failed to wait for logcat output")
			return
		}
		select {
		case <-notFound:
			result <- nil
		default:
			result <- errors.New("mojo connection error is detected")
		}
	}(done)

	// TODO(b:234668738): There are some edge cases which this test cannot catch the actual failure. For example,
	// case #1:
	//
	// 1. A key (e.g. back button) is pressed that closes the activity
	// 2. focusField() runs before the key #1 is processed by Android and succeeds, because there's no wait
	// 3. A key that leads to ARC crash or mojo disconnection is sent, but it's not sent to Android
	// 4. Because it's not received by Android, it fails to catch the regression
	//
	// case #2:
	//
	// 1. A key that leads to ARC crash or mojo disconnection is sent as a last case
	// 2. As there's no wait after the loop, done is sent to the logcat goroutine before the crash message is received
	// 3. Because the test returns without error, it fails to catch the regression
	s.Log("Start typing all keys")
	defer func() {
		done <- true
	}()
	skipKeys := map[input.EventCode]struct{}{
		// Skip KEY_CAPSLOCK to avoid affecting the following tests by Capslock.
		0x3a: {},
		// Skip KEY_SYSRQ to avoid launching the screenshot tool. The
		// screenshot tool can cause subsequent tests to fail by intercepting
		// mouse clicks.
		0x63: {},
		// Skip KEY_POWER which shuts down the machine.
		0x74: {},
		// Skip KEY_LEFTMETA (0x7d) and KEY_RIGHTMETA (0x7e) which are the search keys to avoid confusing the test.
		0x7d: {},
		0x7e: {},
	}
	for scancode := input.EventCode(0x01); scancode < 0x220; scancode++ {
		if _, exist := skipKeys[scancode]; exist || (scancode >= 0x80 && scancode < 0x160) {
			continue
		}
		// Check whether the mojo connection is already broken or not.
		select {
		case err := <-result:
			s.Fatalf("ArcInputMethod mojo connection is broken before typing %d: %v", scancode, err)
		default:
		}

		if err := focusField(); err != nil {
			// Cannot find the text field. Restart the activity.
			act.Stop(ctx, tconn)
			if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
				s.Fatalf("Failed to restart the activity before typing %d: %v", scancode, err)
			}
			if err := focusField(); err != nil {
				s.Fatalf("Failed to focus the field before typing %d: %v", scancode, err)
			}
		}

		if err := kb.TypeKey(ctx, scancode); err != nil {
			s.Fatalf("Failed to send the scancode %d: %v", scancode, err)
		}
	}
	s.Log("Finish typing all keys")

	done <- true
	if err := <-result; err != nil {
		s.Fatal("ArcInputMethod mojo connection is broken while typing test: ", err)
	}
}

func PhysicalKeyboardAllkeys(ctx context.Context, s *testing.State) {
	const apk = "ArcKeyboardTest.apk"

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	physicalKeyboardAllKeycodesTypingTest(ctx, a, tconn, d, kb, s)
}
