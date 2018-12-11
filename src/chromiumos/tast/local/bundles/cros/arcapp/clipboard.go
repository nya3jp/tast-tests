package arcapp

import (
	"context"
	"fmt"
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
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"ArcClipboardTest.apk",
			"clipboard.html",
			"clipboard.js",
			"manifest.json",
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
		pastID             = "org.chromium.arc.testapp.clipboard:id/paste_button"
		editTextID         = "org.chromium.arc.testapp.clipboard:id/edit_message"
		textViewID         = "org.chromium.arc.testapp.clipboard:id/text_view"
		writeTextBtnID     = "org.chromium.arc.testapp.clipboard:id/write_text_button"
		writeHTMLBtnID     = "org.chromium.arc.testapp.clipboard:id/write_html_button"
		observerEnableID   = "org.chromium.arc.testapp.clipboard:id/enable_observer_button"
		observerDisableID  = "org.chromium.arc.testapp.clipboard:id/disable_observer_button"
		observerTextViewID = "org.chromium.arc.testapp.clipboard:id/observer_view"
	)
	extensionFilesNames := []string{
		"clipboard.html",
		"clipboard.js",
		"manifest.json",
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	mustReturn := func(val string, err error) string {
		if err != nil {
			s.Fatal(err)
		}
		return val
	}

	assureEqual := func(err error, expected string, chrome string, android string) {
		if expected != android || expected != chrome {
			s.Fatal(err, " Expected: "+expected+" Chrome: "+chrome+" Android: "+android)
		}
	}

	runJS := func(conn *chrome.Conn, cmd string, errMsg string) {
		var isSuccess bool
		if err := conn.Eval(ctx, cmd, &isSuccess); err != nil {
			s.Fatal("Fail to run: "+cmd+" ", err)
		} else if !isSuccess {
			s.Fatal(errMsg)
		}
	}

	runJSReturn := func(conn *chrome.Conn, cmd string) string {
		var val string
		if err := conn.Eval(ctx, cmd, &val); err != nil {
			s.Fatal("Fail to run "+cmd, err)
		}
		return val
	}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.AddOnExtension(s, extensionFilesNames))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	copyTextFromChromeToAndroid := func(conn *chrome.Conn, d *ui.Device) {
		expected := "Here is some default text"
		// Get text from Chrome.
		valueToCopyCmd := "document.getElementById(\"text_area\").value;"
		chromeText := runJSReturn(conn, valueToCopyCmd)
		// Copy text to clipboard.
		copyCmd := fmt.Sprintf("copy_text_to_clipboard(\"%s\")", chromeText)
		runJS(conn, copyCmd, "Fail to copy text to clipboard")
		// Paste text to Android.
		must(d.Object(ui.ID(pastID)).Click(ctx))
		androidText := mustReturn(d.Object(ui.ID(editTextID)).GetText(ctx))
		assureEqual(errors.New("Fail to copy from Chrome to Android"), expected, chromeText, androidText)
	}

	copyTextFromAndroidToChrome := func(conn *chrome.Conn, d *ui.Device) {
		expected := "Test Text 1234"
		// Get text from Android and copy it to clipboard.
		must(d.Object(ui.ID(writeTextBtnID)).Click(ctx))
		must(d.Object(ui.ID(copyID)).Click(ctx))
		androidText := mustReturn(d.Object(ui.ID(editTextID)).GetText(ctx))
		// Paste text from clipboard to Chrome.
		chromeText := runJSReturn(conn, "paste_text_from_clipboard()")
		assureEqual(errors.New("Fail to copy text from Android to Chrome"), expected, chromeText, androidText)
	}

	copyHTMLFromChromeToAndroid := func(conn *chrome.Conn, d *ui.Device) {
		expected, chromeHTML := "<b>bold</b><i>italics</i>", "<b>bold</b><i>italics</i>"
		// Copy HTML from Chrome to clipboard.
		copyCmd := fmt.Sprintf("copy_html_to_clipboard('%s')", chromeHTML)
		runJS(conn, copyCmd, "Fail to copy HTML to clipboard.")
		// Paste HTML from clipboard to Android.
		must(d.Object(ui.ID(pastID)).Click(ctx))
		androidHTML := mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		assureEqual(errors.New("Fail to copy HTML from Chrome to Android"), expected, chromeHTML, androidHTML)
	}

	copyHTMLFromAndroidToChrome := func(conn *chrome.Conn, d *ui.Device) {
		expected := "<p dir=\"ltr\">test <b>HTML</b> 1234</p>"
		// Write HTML and copy HTML from Android to clipboard.
		must(d.Object(ui.ID(writeHTMLBtnID)).Click(ctx))
		must(d.Object(ui.ID(copyID)).Click(ctx))
		androidHTML := mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		// Paste HTML from clipboard to Chrome.
		chromeHTML := runJSReturn(conn, "paste_html_from_clipboard()")
		assureEqual(errors.New("Fail to copy HTML from Android to Chrome"), expected, chromeHTML, androidHTML)
	}

	copyHTMLFromChromeToAndroidObserver := func(conn *chrome.Conn, d *ui.Device) {
		observerReady := "Observer ready"
		expected, chromeHTML := "<b>observer</b> should paste this", "<b>observer</b> should paste this"
		// Enable observer and wait for it ready to prevent a possible race.
		var observerPollOpt *testing.PollOptions = &testing.PollOptions{Interval: 10 * time.Millisecond}
		must(d.Object(ui.ID(observerEnableID)).Click(ctx))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if mustReturn(d.Object(ui.ID(observerTextViewID)).GetText(ctx)) != observerReady {
				return errors.New("observer is not ready")
			}
			return nil
		}, observerPollOpt); err != nil {
			s.Fatal("Time out for waiting for Observer ready. ")
		}
		// Invoke 'copy' after observer ready. Copy HTML from Chrome to clipboard.
		copyCmd := fmt.Sprintf("copy_html_to_clipboard('%s')", chromeHTML)
		runJS(conn, copyCmd, "Fail to copy HTML to clipboard.")
		// Paste HTML from clipboard to Android.
		must(d.Object(ui.ID(pastID)).Click(ctx))
		androidHTML := mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
		// Disable observer.
		must(d.Object(ui.ID(observerDisableID)).Click(ctx))
		assureEqual(errors.New("Fail to copy HTML from Chrome to Android with observer enabled"), expected, chromeHTML, androidHTML)
	}

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		// Wait for App showing up.
		must(d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx))

		conn, err := cr.TestAddOnAPIConn(ctx, "clipboard.html")
		if err != nil {
			s.Fatal("Failed to open my test api: ", err)
		}
		defer conn.Close()

		copyTextFromChromeToAndroid(conn, d)
		copyTextFromAndroidToChrome(conn, d)
		copyHTMLFromChromeToAndroid(conn, d)
		copyHTMLFromAndroidToChrome(conn, d)
		copyHTMLFromChromeToAndroidObserver(conn, d)
	})
}
