// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndroidIMEInBrowser,
		Desc:         "Checks Android IME in a browser window",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcInputMethodTest.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedInTabletMode(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedInTabletMode(),
		}},
	})
}

func getThirdPartyInputMethodID(ctx context.Context, tconn *chrome.Conn, pkg string) (string, error) {
	var lst []struct {
		ID string `json:"id"`
	}
	if err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.languageSettingsPrivate.getInputMethodLists(function(imeLists) {
		    resolve(imeLists.thirdPartyExtensionImes);
		  });
		})`, &lst); err != nil {
		return "", err
	}
	for _, im := range lst {
		if strings.Contains(im.ID, pkg) {
			return im.ID, nil
		}
	}
	return "", errors.Errorf("%s not found in the list", pkg)
}

func isKeyboardShown(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	var shown bool
	if err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.automation.getDesktop(function(root){
		    const keyboard = root.find({ attributes: { className: 'ArcVirtualKeyboardContainer' }});
		    resolve(!!(keyboard && keyboard.children.length > 0));
		  });
		})`, &shown); err != nil {
		return false, err
	}
	return shown, nil
}

func AndroidIMEInBrowser(ctx context.Context, s *testing.State) {
	const (
		apk         = "ArcInputMethodTest.apk"
		pkg         = "org.chromium.arc.testapp.ime"
		settingsPkg = "com.android.settings"
	)

	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	dev, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer dev.Close()

	s.Log("Installing IME service")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing IME: ", err)
	}

	s.Log("Waiting for ARC Test IME")
	var id string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		id, err = getThirdPartyInputMethodID(ctx, tconn, pkg)
		return err
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for ARC Test IME: ", err)
	}

	s.Log("Enabling ARC Test IME")
	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.languageSettingsPrivate.addInputMethod(%q);`, id), nil); err != nil {
		s.Fatal("Failed to enable ARC Test IME: ", err)
	}

	// Two confirmation dialogs are shown by Android framework.
	for i := 0; i < 2; i++ {
		// Use ui.Text instead of ui.ID as OK button does not have ID.
		btn := dev.Object(ui.Text("OK"), ui.PackageName(settingsPkg))
		if err := btn.WaitForExists(ctx, time.Minute); err != nil {
			s.Fatal("Failed to find OK button: ", err)
		}
		if err := btn.Click(ctx); err != nil {
			s.Fatal("Failed to click OK button: ", err)
		}
	}
	if dev.WaitForIdle(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	s.Log("Activating ARC Test IME")
	if err := tconn.Eval(ctx,
		fmt.Sprintf(`chrome.inputMethodPrivate.setCurrentInputMethod(%q);`, id), nil); err != nil {
		s.Fatal("Failed to activate ARC Test IME: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var actualID string
		if err := tconn.EvalPromise(ctx,
			`new Promise(function(resolve, reject) {
			  chrome.inputMethodPrivate.getCurrentInputMethod(function(id) {
			    resolve(id);
			  });
			})`, &actualID); err != nil {
			return err
		}
		if actualID != id {
			return errors.Errorf("got input method ID %q while expecting %q", actualID, id)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond}); err != nil {
		s.Fatal("Failed to activate ARC Test IME: ", err)
	}

	// Show a page with a text field that autofocuses. Turn off autocorrect as it
	// can interfere with the test.
	const html = `<input type="text" id="text" autocorrect="off" autofocus/>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer for test page failed: ", err)
	}
	defer conn.Close()

	s.Log("Waiting for the text field to focus")
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('text') === document.activeElement`); err != nil {
		s.Fatal("Failed to wait for text field to focus: ", err)
	}

	s.Log("Showing the virtual keyboard")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if shown, err := isKeyboardShown(ctx, tconn); err != nil {
			return testing.PollBreak(err)
		} else if shown {
			return nil
		}
		// Repeatedly call showVirtualKeyboardIfEnabled until isKeyboardShown returns true.
		// Usually it requires only one call of showVirtualKeyboardIfEnabled, but on rare occasions it requires
		// multiple ones e.g. the function was called right in the middle of IME switch.
		if err := tconn.Eval(ctx, `chrome.autotestPrivate.showVirtualKeyboardIfEnabled();`, nil); err != nil {
			return testing.PollBreak(err)
		}
		return errors.New("virtual keyboard still not shown")
	}, nil); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	info, err := dev.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get device info: ", err)
	}

	// You can only type "a" from the test IME.
	const expected = "aaaa"

	s.Log("Trying to press the button in ARC Test IME")
	for i := 0; i < len(expected); i++ {
		if dev.WaitForIdle(ctx, time.Minute); err != nil {
			s.Fatal("Failed to wait for idle: ", err)
		}

		// Click on the left bottom directly, as the virtual keyboard is not visible to UIAutomator.
		// Subtract 1 from the height, as [0, DisplayHeight - 1] is valid y range.
		if err := dev.Click(ctx, 0, info.DisplayHeight-1); err != nil {
			s.Fatal("Failed to click: ", err)
		}
	}

	s.Log("Waiting for the text field to have the correct contents")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const expr = `document.getElementById('text').value`
		var actual string
		if err := conn.Eval(ctx, expr, &actual); err != nil {
			return err
		}
		if actual != expected {
			return errors.Errorf("got input %q from field after typing %q", actual, expected)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Error("Failed to get input text: ", err)
	}
}
