// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Clipboard,
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa",
		Contacts:     []string{"ruanc@chromium.org", "niwa@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"ArcClipboardTest.apk",
			"clipboard_extension/background.html",
			"clipboard_extension/background.js",
			"clipboard_extension/manifest.json",
		},
	})
}

func Clipboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcClipboardTest.apk"
		pkg = "org.chromium.arc.testapp.clipboard"
		cls = "org.chromium.arc.testapp.clipboard.ClipboardActivity"

		idPrefix           = "org.chromium.arc.testapp.clipboard:id/"
		titleID            = idPrefix + "text_view"
		title              = "HTML tags goes here"
		copyID             = idPrefix + "copy_button"
		pasteID            = idPrefix + "paste_button"
		editTextID         = idPrefix + "edit_message"
		textViewID         = idPrefix + "text_view"
		writeTextBtnID     = idPrefix + "write_text_button"
		writeHTMLBtnID     = idPrefix + "write_html_button"
		observerEnableID   = idPrefix + "enable_observer_button"
		observerDisableID  = idPrefix + "disable_observer_button"
		observerTextViewID = idPrefix + "observer_view"
		observerReady      = "Observer ready"

		expectedTextFromChrome  = "Here is some default text"
		expectedTextFromAndroid = "Test Text 1234"
		expectedHTMLFromChrome  = "<b>bold</b><i>italics</i>"
		expectedHTMLFromAndroid = `<p dir="ltr">test <b>HTML</b> 1234</p>`
		expectedHTMLObserver    = "<b>observer</b> should paste this"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	mustReturn := func(val string, err error) string {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
		return val
	}

	assertEqual := func(errMsg, chrome, android, expected string) {
		if expected != android || expected != chrome {
			s.Fatalf("%s: Chrome %q, Android %q, expected: %q", errMsg, chrome, android, expected)
		}
	}

	evalBoolJS := func(conn *chrome.Conn, cmd string) {
		var isSuccess bool
		if err := conn.Eval(ctx, cmd, &isSuccess); err != nil {
			s.Fatalf("Failed to eval %q: %v", cmd, err)
		} else if !isSuccess {
			s.Fatalf("Returned false: %q", cmd)
		}
	}

	evalStringJS := func(conn *chrome.Conn, cmd string) string {
		var val string
		if err := conn.Eval(ctx, cmd, &val); err != nil {
			s.Fatalf("Failed to eval %q: %v", cmd, err)
		}
		return val
	}

	extDir, err := ioutil.TempDir("", "tast.arc.Clipboard.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	s.Log("Writing unpacked extension to ", extDir)
	for _, fn := range []string{"manifest.json", "background.html", "background.js"} {
		if err := fsutil.CopyFile(s.DataPath(filepath.Join("clipboard_extension", fn)), filepath.Join(extDir, fn)); err != nil {
			s.Fatalf("Failed to copy file %v: %v", fn, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}
	s.Log("Extension ID is ", extID)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnpackedExtension(extDir))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Connecting to background page")
	bgURL := "chrome-extension://" + extID + "/background.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := conn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
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

	s.Log("Starting app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	// Wait for App showing up.
	must(d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx, 30*time.Second))

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

	// Copy image from Chrome to Android.
	expectedImage := evalStringJS(conn, `document.getElementById('image').src;`)
	evalBoolJS(conn, "copyImageToClipboard()")
	must(d.Object(ui.ID(pasteID)).Click(ctx))
	androidHTML = mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
	// Note: style attribute is added by Chrome before the image is copied to Android.
	imageRegexp := regexp.MustCompile(`^<img id="image" src="(data:[^"]+)" style="[^"]+">$`)
	if match := imageRegexp.FindStringSubmatch(androidHTML); match == nil {
		s.Fatalf("Failed to find image after copying from Chrome to Android: got %q", androidHTML)
	} else if match[1] != expectedImage {
		s.Fatalf("Wrong URL after copying image from Chrome to Android: got %q; want %q", match[1], expectedImage)
	}

	// Copy HTML from Chrome to Android with Observer.
	// Enable observer and wait for it to be ready to prevent a possible race.
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
	chromeHTML = "<b>observer</b> should paste this"
	copyCmd = fmt.Sprintf("copyHtmlToClipboard(%q)", chromeHTML)
	evalBoolJS(conn, copyCmd)
	must(d.Object(ui.ID(pasteID)).Click(ctx))
	androidHTML = mustReturn(d.Object(ui.ID(textViewID)).GetText(ctx))
	// Disable observer.
	must(d.Object(ui.ID(observerDisableID)).Click(ctx))
	assertEqual("Failed to copy HTML from Chrome to Android with observer enabled", chromeHTML, androidHTML, expectedHTMLObserver)

	// TODO(ruanc): Copying big text (500Kb) is blocked by https://bugs.chromium.org/p/chromium/issues/detail?id=916882
}
