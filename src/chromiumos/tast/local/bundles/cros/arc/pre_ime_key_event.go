// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PreImeKeyEvent,
		Desc:         "Checks View.onKeyPreIme() works on Android apps",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func PreImeKeyEvent(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=ArcPreImeKeyEventSupport"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	const (
		apk          = "ArcKeyboardTest.apk"
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".CheckKeyPreImeActivity"

		editTextID         = pkg + ":id/text"
		lastKeyDownLabelID = pkg + ":id/last_key_down"
		lastKeyUpLabelID   = pkg + ":id/last_key_up"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx)

	editText := d.Object(ui.ID(editTextID))
	if err := editText.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the text field: ", err)
	}
	if err := editText.Click(ctx); err != nil {
		s.Fatal("Failed to click the text field: ", err)
	}
	if err := editText.SetText(ctx, ""); err != nil {
		s.Log("Failed to empty the text field: ", err)
	}

	if err := d.Object(ui.ID(editTextID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus a text field: ", err)
	}

	//
	keyDownLabel := d.Object(ui.ID(lastKeyDownLabelID))
	keyUpLabel := d.Object(ui.ID(lastKeyUpLabelID))
	for _, key := range []struct {
		Key      string
		Expected int
	}{
		{"a", 29},  // AKEYCODE_A
		{"b", 30},  // AKEYCODE_B
		{"c", 31},  // AKEYCODE_C
		{"\b", 67}, // AKEYCODE_DEL
		{"\n", 66}, // AKEYCODE_ENTER
	} {
		if err := kb.Type(ctx, key.Key); err != nil {
			s.Fatalf("Failed to type %q", key.Key)
		}
		// Check the input field after each keystroke.
		expectedText := fmt.Sprintf("key down: keyCode=%d", key.Expected)
		if err := d.Object(ui.ID(lastKeyDownLabelID), ui.Text(expectedText)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := keyDownLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, expectedText)
			}
		}
		expectedText = fmt.Sprintf("key up: keyCode=%d", key.Expected)
		if err := d.Object(ui.ID(lastKeyUpLabelID), ui.Text(expectedText)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := keyUpLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, expectedText)
			}
		}
	}
}
