// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Clipboard,
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"ArcClipboardTest.apk",
			"clipboard/background.html",
			"clipboard/background.js",
			"clipboard/manifest.json",
		},
	})
}

func Clipboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcClipboardTest.apk"
		pkg = "org.chromium.arc.testapp.clipboard"
		cls = "org.chromium.arc.testapp.clipboard.ClipboardActivity"

		titleID            = "org.chromium.arc.testapp.clipboard:id/text_view"
		title              = "HTML tags goes here"
		copyID             = "org.chromium.arc.testapp.clipboard:id/copy_button"
		pasteID            = "org.chromium.arc.testapp.clipboard:id/paste_button"
		editTextID         = "org.chromium.arc.testapp.clipboard:id/edit_message"
		textViewID         = "org.chromium.arc.testapp.clipboard:id/text_view"
		writeTextBtnID     = "org.chromium.arc.testapp.clipboard:id/write_text_button"
		writeHTMLBtnID     = "org.chromium.arc.testapp.clipboard:id/write_html_button"
		observerEnableID   = "org.chromium.arc.testapp.clipboard:id/enable_observer_button"
		observerDisableID  = "org.chromium.arc.testapp.clipboard:id/disable_observer_button"
		observerTextViewID = "org.chromium.arc.testapp.clipboard:id/observer_view"

		expectedTextFromChrome  = "Here is some default text"
		expectedTextFromAndroid = "Test Text 1234"
		expectedHTMLFromChrome  = "<b>bold</b><i>italics</i>"
		expectedHTMLFromAndroid = `<p dir="ltr">test <b>HTML</b> 1234</p>`
		expectedHTMLObserver    = "<b>observer</b> should paste this"
		observerReady           = "Observer ready"
	)
	extensionFilesNames := []string{
		"background.html",
		"background.js",
		"manifest.json",
	}

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed:", err)
		}
	}

	mustReturn := func(val string, err error) string {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
		return val
	}

	assertEqual := func(errMsg string, chrome string, android string, expected string) {
		if expected != android || expected != chrome {
			s.Fatalf("%s: Chrome %q, Android %q, expected: %q", errMsg, chrome, android, expected)
		}
	}

	evalBoolJS := func(conn *chrome.Conn, cmd string) {
		var isSuccess bool
		if err := conn.Eval(ctx, cmd, &isSuccess); err != nil {
			s.Fatal("Failed to eval %q: %v", cmd, err)
		} else if !isSuccess {
			s.Fatalf("%q returned false")
		}
	}

	evalStringJS := func(conn *chrome.Conn, cmd string) string {
		var val string
		if err := conn.Eval(ctx, cmd, &val); err != nil {
			s.Fatalf("Failed to eval %q: %v", cmd, err)
		}
		return val
	}

	dir, _ := filepath.Split(s.DataPath("clipboard/manifest.json"))
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.InstallExtension(dir, extensionFilesNames))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		// Wait for App showing up.
		must(d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx))

		conn, err := cr.TestAddOnAPIConn(ctx, "background.html")
		if err != nil {
			s.Fatal("Failed to open my test api: ", err)
		}
		defer conn.Close()

		// Copy text from Chrome to Android.
		valToCopyCmd := `document.getElementById('text_area').value;`
		chromeText := evalStringJS(conn, valToCopyCmd)
		copyCmd := fmt.Sprintf("copyTextToClipboard(%q)", chromeText)
		evalBoolJS(conn, copyCmd)
		must(d.Object(ui.ID(pasteID)).Click(ctx))
		androidText := mustReturn(d.Object(ui.ID(editTextID)).GetText(ctx))
		assertEqual("Failed to copy from Chrome to Android", chromeText, androidText, expectedTextFromChrome)

		// Copy text from Android to Chrome.
		must(d.Object(ui.ID(writeTextBtnID)).Click(ctx))
		must(d.Object(ui.ID(copyID)).Click(ctx))
		androidText = mustReturn(d.Object(ui.ID(editTextID)).GetText(ctx))
		chromeText = evalStringJS(conn, "pasteTextFromClipboard()")
		assertEqual("Failed to copy text from Android to Chrome", chromeText, androidText, expectedTextFromAndroid)

		// Copy HTML from Chrome to Android.
		chromeHTML := "<b>bold</b><i>italics</i>"
		copyCmd = fmt.Sprintf("copyHtmlToClipboard(%q)", chromeHTML)
		evalBoolJS(conn, copyCmd)
		must(d.Object(ui.ID(pasteID)).Click(ctx))
		androidHTML := mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		assertEqual("Failed to copy HTML from Chrome to Android", chromeHTML, androidHTML, expectedHTMLFromChrome)

		// Copy HTML From Android to Chrome.
		must(d.Object(ui.ID(writeHTMLBtnID)).Click(ctx))
		must(d.Object(ui.ID(copyID)).Click(ctx))
		androidHTML = mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		chromeHTML = evalStringJS(conn, "pasteHtmlFromClipboard()")
		assertEqual("Failed to copy HTML from Android to Chrome", chromeHTML, androidHTML, expectedHTMLFromAndroid)

		// Copy HTML from Chrome to Android with Observer.
		chromeHTML = "<b>observer</b> should paste this"
		// Enable observer and wait for it ready to prevent a possible race.
		must(d.Object(ui.ID(observerEnableID)).Click(ctx))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if mustReturn(d.Object(ui.ID(observerTextViewID)).GetText(ctx)) != observerReady {
				return errors.New("observer is not ready")
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed waiting for observer readiness: ", err)
		}
		// Invoke 'copy' after observer ready.
		copyCmd = fmt.Sprintf("copyHtmlToClipboard(%q)", chromeHTML)
		evalBoolJS(conn, copyCmd)
		must(d.Object(ui.ID(pasteID)).Click(ctx))
		androidHTML = mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		// Disable observer.
		must(d.Object(ui.ID(observerDisableID)).Click(ctx))
		assertEqual("Failed to copy HTML from Chrome to Android with observer enabled", chromeHTML, androidHTML, expectedHTMLObserver)
	})
}
