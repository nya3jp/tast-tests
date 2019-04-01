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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome_login"},
		Data:         []string{"ArcInputMethodTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

type inputMethod struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

func getArcTestIME(ctx context.Context, tconn *chrome.Conn) (*inputMethod, error) {
	l := []inputMethod{}
	if err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
	    chrome.languageSettingsPrivate.getInputMethodLists(function(imeLists) {
	      resolve(imeLists.thirdPartyExtensionImes);
	    });
	  })`, &l); err != nil {
		return nil, err
	}
	for _, im := range l {
		if im.DisplayName == "ARC Test IME" {
			return &im, nil
		}
	}
	return nil, errors.New("ARC Test IME not found in the list")
}

func isArcTestIMEEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	l := []struct {
		Name string `json:"name"`
	}{}
	if err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
	    chrome.inputMethodPrivate.getInputMethods(function(imeList) {
	      resolve(imeList);
	    });
	  })`, &l); err != nil {
		return false, err
	}
	for _, im := range l {
		if im.Name == "ARC Test IME" {
			return true, nil
		}
	}
	return false, nil
}

func AndroidIMEInBrowser(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcInputMethodTest.apk"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Installing IME service")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing IME: ", err)
	}

	s.Log("Waiting for ARC Test IME")

	var im *inputMethod
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		im, err = getArcTestIME(ctx, tconn)
		return err
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for ARC Test IME: ", err)
	}

	if err := tconn.Eval(ctx, fmt.Sprintf(`chrome.languageSettingsPrivate.addInputMethod("%s");`, im.ID), nil); err != nil {
		s.Fatal("Failed to enable ARC Test IME: ", err)
	}

	for i := 0; i < 2; i++ {
		btn := d.Object(ui.Text("OK"))
		if err := btn.WaitForExists(ctx, time.Minute); err != nil {
			s.Fatal("Failed to find OK button: ", err)
		}
		if err := btn.Click(ctx); err != nil {
			s.Fatal("Failed to click OK button: ", err)
		}
		if d.WaitForIdle(ctx, time.Minute); err != nil {
			s.Fatal("Failed to wait for idle: ", err)
		}
	}

	if ok, err := isArcTestIMEEnabled(ctx, tconn); err != nil {
		s.Fatal("Failed to check: ", err)
	} else if !ok {
		s.Fatal("ARC Test IME is not enabled")
	}

	if err := tconn.Eval(ctx, fmt.Sprintf(`chrome.inputMethodPrivate.setCurrentInputMethod("%s");`, im.ID), nil); err != nil {
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

	// Wait for the text field to focus.
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('text') === document.activeElement`); err != nil {
		s.Fatal("Failed to wait for text field to focus: ", err)
	}
	if err := tconn.Eval(ctx, `chrome.autotestPrivate.showVirtualKeyboardIfEnabled();`, nil); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get device info: ", err)
	}

	s.Log("Trying to press")

	for i := 0; i < 4; i++ {
		if d.WaitForIdle(ctx, time.Minute); err != nil {
			s.Fatal("Failed to wait for idle: ", err)
		}
		if err := d.Click(ctx, 100, info.DisplayHeight-200); err != nil {
			s.Fatal("Failed to click: ", err)
		}
	}

	s.Log("Waiting for the text field to have the correct contents")
	if err := conn.WaitForExpr(ctx,
		`document.getElementById('text').value === 'aaaa'`); err != nil {
		s.Fatal("Failed to get the contents of the text field: ", err)
	}
}
