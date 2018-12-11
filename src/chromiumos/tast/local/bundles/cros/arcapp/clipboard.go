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
			"background.html",
			"background.js",
			"manifest.json",
		},
	})
}

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
	//primaryClipBtnID   = "org.chromium.arc.testapp.clipboard:id/has_clipboard_button"
	//descriptionBtnID   = "org.chromium.arc.testapp.clipboard:id/get_description_button"

)

func Clipboard(ctx context.Context, s *testing.State) {
	extensionFilesNames := []string{
		"background.html",
		"background.js",
		"manifest.json",
	}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.AddOnExtension(s, extensionFilesNames))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		// Wait for App showing up.
		must(s, d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx))

		conn, err := cr.TestAddOnAPIConn(ctx, "background.html")
		if err != nil {
			s.Fatal("Failed to open my test api: ", err)
		}
		defer conn.Close()

		copyTextFromChromeToAndroid(ctx, s, conn, d)
		copyTextFromAndroidToChrome(ctx, s, conn, d)
		copyHTMLFromChromeToAndroid(ctx, s, conn, d)
		copyHTMLFromAndroidToChrome(ctx, s, conn, d)
		copyHTMLFromChromeToAndroidObserver(ctx, s, conn, d)
		/* Skip copy with large text temporarily due to writeLimiter in *rpcc.Conn */
		// copyTextFromChromeToAndroidOverflow(ctx, s, conn, d)
		// copyTextFromChromeToAndroidOverflowBis(ctx, s, conn, d)
	})
}

func runJS(ctx context.Context, s *testing.State, conn *chrome.Conn, cmd string, errMsg string) {
	var isSuccess bool
	if err := conn.Eval(ctx, cmd, &isSuccess); err != nil {
		s.Fatal("Fail to run: "+cmd+" ", err)
	} else if !isSuccess {
		s.Fatal(errMsg)
	}
}

func runJSWithReturn(ctx context.Context, s *testing.State, conn *chrome.Conn, cmd string) string {
	var val string
	if err := conn.Eval(ctx, cmd, &val); err != nil {
		s.Fatal("Fail to run "+cmd, err)
	}
	return val
}

func must(s *testing.State, err error) {
	if err != nil {
		s.Fatal(err)
	}
}

func mustGetTextValByID(ctx context.Context, s *testing.State, d *ui.Device, id string) string {
	var val string
	var err error
	if val, err = d.Object(ui.ID(id)).GetText(ctx); err != nil {
		s.Fatal(err)
	}
	return val
}

func assureEqual(s *testing.State, err error, expected string, chrome string, android string) {
	if expected != android || expected != chrome {
		s.Fatal(err, " Expected: "+expected+" Chrome: "+chrome+" Android: "+android)
	}

}

func copyTextFromChromeToAndroid(ctx context.Context, s *testing.State, conn *chrome.Conn,
	d *ui.Device) {
	expected := "Here is some default text"

	// Get text from Chrome.
	valueToCopyCmd := "document.getElementById(\"text_area\").value;"
	chromeText := runJSWithReturn(ctx, s, conn, valueToCopyCmd)

	// Copy text to clipboard.
	copyCmd := fmt.Sprintf("copy_text_to_clipboard(\"%s\")", chromeText)
	runJS(ctx, s, conn, copyCmd, "Fail to copy text to clipboard")

	// Paste text to Android.
	must(s, d.Object(ui.ID(pastID)).Click(ctx))
	androidText := mustGetTextValByID(ctx, s, d, editTextID)

	assureEqual(s, errors.New("fail to copy from Chrome to Android"), expected, chromeText, androidText)
}

func copyTextFromAndroidToChrome(ctx context.Context, s *testing.State, conn *chrome.Conn, d *ui.Device) {
	expected := "Test Text 1234"

	// Get text from Android and copy it to clipboard.
	must(s, d.Object(ui.ID(writeTextBtnID)).Click(ctx))
	must(s, d.Object(ui.ID(copyID)).Click(ctx))
	androidText := mustGetTextValByID(ctx, s, d, editTextID)

	// Paste text from clipboard to Chrome.
	chromeText := runJSWithReturn(ctx, s, conn, "paste_text_from_clipboard()")

	assureEqual(s, errors.New("Fail to copy text from Android to Chrome"), expected, chromeText, androidText)
}

func copyHTMLFromChromeToAndroid(ctx context.Context, s *testing.State, conn *chrome.Conn,
	d *ui.Device) {
	expected := "<b>bold</b><i>italics</i>"

	// Copy HTML from Chrome to clipboard.
	chromeHTML := "<b>bold</b><i>italics</i>"
	copyCmd := fmt.Sprintf("copy_html_to_clipboard('%s')", chromeHTML)
	runJS(ctx, s, conn, copyCmd, "Fail to copy HTML to clipboard.")

	// Paste HTML from clipboard to Android.
	must(s, d.Object(ui.ID(pastID)).Click(ctx))
	androidHTML := mustGetTextValByID(ctx, s, d, textViewID)

	assureEqual(s, errors.New("fail to copy HTML from Chrome to Android"), expected, chromeHTML, androidHTML)
}

func copyHTMLFromAndroidToChrome(ctx context.Context, s *testing.State, conn *chrome.Conn, d *ui.Device) {
	expected := "<p dir=\"ltr\">test <b>HTML</b> 1234</p>"

	// Write HTML and copy HTML from Android to clipboard.
	must(s, d.Object(ui.ID(writeHTMLBtnID)).Click(ctx))
	must(s, d.Object(ui.ID(copyID)).Click(ctx))
	androidHTML := mustGetTextValByID(ctx, s, d, textViewID)

	// Paste HTML from clipboard to Chrome.
	chromeHTML := runJSWithReturn(ctx, s, conn, "paste_html_from_clipboard()")

	assureEqual(s, errors.New("fail to copy HTML from Android to Chrome"), expected, chromeHTML, androidHTML)
}

func copyHTMLFromChromeToAndroidObserver(ctx context.Context, s *testing.State, conn *chrome.Conn,
	d *ui.Device) {
	observerReady := "Observer ready"
	expected := "<b>observer</b> should paste this"
	chromeHTML := "<b>observer</b> should paste this"

	// Enable observer and wait for it ready to prevent a possible race.
	var observerPollOpt *testing.PollOptions = &testing.PollOptions{Interval: 10 * time.Millisecond}
	must(s, d.Object(ui.ID(observerEnableID)).Click(ctx))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if mustGetTextValByID(ctx, s, d, observerTextViewID) != observerReady {
			return errors.New("observer is not ready")
		}
		return nil
	}, observerPollOpt); err != nil {
		s.Fatal("Time out for waiting for Observer ready. ")
	}

	// Invoke 'copy' after observer ready. Copy HTML from Chrome to clipboard.
	copyCmd := fmt.Sprintf("copy_html_to_clipboard('%s')", chromeHTML)
	runJS(ctx, s, conn, copyCmd, "Fail to copy HTML to clipboard.")

	// Paste HTML from clipboard to Android.
	must(s, d.Object(ui.ID(pastID)).Click(ctx))
	androidHTML := mustGetTextValByID(ctx, s, d, textViewID)

	// Disable observer.
	must(s, d.Object(ui.ID(observerDisableID)).Click(ctx))

	assureEqual(s, errors.New("fail to copy HTML from Chrome to Android with observer enabled"), expected, chromeHTML, androidHTML)
}

// Skip the copy large text due to the writeLimiter in *rpcc.conn.
/*
func copyTextFromChromeToAndroidOverflow(ctx context.Context, s *testing.State, conn *chrome.Conn, d *ui.Device)  {
	chromeText := "old clip"
	expected := "old clip"

	// Copy small text from Chrome to clipboard.
	copyCmd := fmt.Sprintf("copy_text_to_clipboard(\"%s\")", expected)
	runJSWithCheck(ctx, s, conn, copyCmd, "Fail to copy text to clipboard.")
	must(s, d.Object(ui.ID(pastID)).Click(ctx))

	// Copies 500Kb in the buffer. It should be filtered by Chrome since it is bigger than the max
	// Binder Parcel size.
	copyCmd = fmt.Sprintf("copy_text_to_clipboard(\"%s\")", strings.Repeat("A",  1024 * 500))
	//runJSWithCheck(ctx, s, conn, copyCmd, "Fail to copy large text to clipboard.")
	var isCopy bool
	if err := conn.Eval(ctx, copyCmd, &isCopy); err != nil {
		s.Fatal("Failed to run copy_text_to_clipboard(): ", err)
	} else if !isCopy {
		s.Fatal("Failed to copy text to clipboard.")
	}
	must(s, d.Object(ui.ID(pastID)).Click(ctx))

	androidText := mustGetTextValById(ctx, s, d, textViewID)
	assureEqual(s, errors.New("Fail to copy text from Chrome to Android with overflow."), expected, chromeText, androidText)
}

func copyTextFromChromeToAndroidOverflowBis(ctx context.Context, s *testing.State, conn *chrome.Conn, d *ui.Device)  {
	// Copy small text.
	copyCmd := "copy_text_to_clipboard('hello')"
	runJSWithCheck(ctx, s, conn, copyCmd, "Fail to copy small text to clipboard.")

	must(s, d.Object(ui.ID(primaryClipBtnID)).Click(ctx))
	has_clip := mustGetTextValById(ctx, s, d, observerTextViewID)

	must(s, d.Object(ui.ID(descriptionBtnID)).Click(ctx))
	get_desc := mustGetTextValById(ctx, s, d, observerTextViewID)

	var validErr []string
	expect_has_clip := "hasClipboard() = true"
	expect_get_desc := "getClipDescription() = true"
	if expect_has_clip != has_clip {
		msg := fmt.Sprintf("expected %s, got %s", expect_has_clip, has_clip)
		validErr = append(validErr, msg)
	}
	if expect_get_desc != get_desc {
		msg := fmt.Sprintf("expected %s, got %s", expect_get_desc, get_desc)
		validErr = append(validErr, msg)
	}
	if len(validErr) > 0 {
		s.Fatal("Valid clips expected. Instead received: ", strings.Join(validErr, ". "))
	}

    // Copies 500Kb in the buffer.
	copyCmd = fmt.Sprintf("copy_text_to_clipboard(\"%s\")", strings.Repeat("A",  1024 * 500))
	runJSWithCheck(ctx, s, conn, copyCmd, "Fail to copy large text from Chrome to clipboard.")

	must(s, d.Object(ui.ID(primaryClipBtnID)).Click(ctx))
	has_clip = mustGetTextValById(ctx, s, d, observerTextViewID)

	must(s, d.Object(ui.ID(descriptionBtnID)).Click(ctx))
	get_desc = mustGetTextValById(ctx, s, d, observerTextViewID)

	validErr = validErr[:0]
	if expect_has_clip != has_clip {
		msg := fmt.Sprintf("expected %s, got %s", expect_has_clip, has_clip)
		validErr = append(validErr, msg)
	}
	if expect_get_desc != get_desc {
		msg := fmt.Sprintf("expected %s, got %s", expect_get_desc, get_desc)
		validErr = append(validErr, msg)
	}
	if len(validErr) > 0 {
		s.Fatal("Valid clips expected. Instead received: ", strings.Join(validErr, ". "))
	}
}
*/
