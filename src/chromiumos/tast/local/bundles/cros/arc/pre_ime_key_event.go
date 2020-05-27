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

type testKeyStroke struct {
	Key                 string
	ExpectedPreImeKey   int
	ExpectedPostImeKey  int
	ExpectedTextOnField string
}

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

func getExpectedKeyLabelText(keyCode int) string {
	if keyCode == 0 {
		return "null"
	}
	return fmt.Sprintf("key down: keyCode=%d", keyCode)
}

func testPreImeKeyEvent(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, kb *input.KeyboardEventWriter, s *testing.State, fieldID string, keystrokes []testKeyStroke) {
	const (
		apk          = "ArcKeyboardTest.apk"
		pkg          = "org.chromium.arc.testapp.keyboard"
		activityName = ".CheckKeyPreImeActivity"

		lastKeyDownLabelID   = pkg + ":id/last_key_down"
		lastPreImeKeyLabelID = pkg + ":id/last_pre_ime_key"
	)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	textField := d.Object(ui.ID(fieldID))
	if err := textField.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the text field: ", err)
	}
	if err := textField.Click(ctx); err != nil {
		s.Fatal("Failed to click the text field: ", err)
	}
	if err := textField.SetText(ctx, ""); err != nil {
		s.Fatal("Failed to empty the text field: ", err)
	}

	if err := d.Object(ui.ID(fieldID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to focus a text field: ", err)
	}

	preImeKeyLabel := d.Object(ui.ID(lastPreImeKeyLabelID))
	keyDownLabel := d.Object(ui.ID(lastKeyDownLabelID))
	for _, key := range keystrokes {
		if err := kb.Type(ctx, key.Key); err != nil {
			s.Fatalf("Failed to type %q", key.Key)
		}
		if err := d.Object(ui.ID(lastPreImeKeyLabelID), ui.Text(getExpectedKeyLabelText(key.ExpectedPreImeKey))).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := preImeKeyLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, getExpectedKeyLabelText(key.ExpectedPreImeKey))
			}
		}
		if err := d.Object(ui.ID(lastKeyDownLabelID), ui.Text(getExpectedKeyLabelText(key.ExpectedPostImeKey))).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := keyDownLabel.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, getExpectedKeyLabelText(key.ExpectedPostImeKey))
			}
		}
		if err := d.Object(ui.ID(fieldID), ui.Text(key.ExpectedTextOnField)).WaitForExists(ctx, 30*time.Second); err != nil {
			if actual, err := textField.GetText(ctx); err != nil {
				s.Fatal("Failed to get text: ", err)
			} else {
				s.Fatalf("Got input %q from field. Expected %q", actual, key.ExpectedTextOnField)
			}
		}
	}
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

		editTextID     = pkg + ":id/text"
		nullEditTextID = pkg + ":id/null_edit"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	for _, tc := range []struct {
		name    string
		fieldID string
		strokes []testKeyStroke
	}{
		{
			// On a normal text field, onKeyPreIme() is called, however, onKeyDown() is not called
			// because IME consumes the key event.
			name:    "normal",
			fieldID: editTextID,
			strokes: []testKeyStroke{
				{"a", 29, 0, "a"},    // AKEYCODE_A
				{"b", 30, 0, "ab"},   // AKEYCODE_B
				{"c", 31, 0, "abc"},  // AKEYCODE_C
				{"\b", 67, 67, "ab"}, // AKEYCODE_DEL
				{"\n", 66, 66, "ab"}, // AKEYCODE_ENTER
			},
		},
		{
			// On a TYPE_NULL text field, both of onKeyPreIme() and onKeyDown() are called.
			name:    "null",
			fieldID: nullEditTextID,
			strokes: []testKeyStroke{
				{"a", 29, 29, "a"},   // AKEYCODE_A
				{"b", 30, 30, "ab"},  // AKEYCODE_B
				{"c", 31, 31, "abc"}, // AKEYCODE_C
				{"\b", 67, 67, "ab"}, // AKEYCODE_DEL
				{"\n", 66, 66, "ab"}, // AKEYCODE_ENTER
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			testPreImeKeyEvent(ctx, tconn, a, d, kb, s, tc.fieldID, tc.strokes)
		})
	}
}
