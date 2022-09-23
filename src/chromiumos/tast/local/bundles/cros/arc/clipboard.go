// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Clipboard,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa",
		Contacts:     []string{"ruanc@chromium.org", "yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"clipboard_image.html"},
		Params: []testing.Param{{
			// b:238260020 - disable aged (>1y) unpromoted informational tests
			// ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			// b:238260020 - disable aged (>1y) unpromoted informational tests
			// ExtraAttr:         []string{"group:mainline", "informational"},
		}},
	})
}

// idPrefix is the prefix to all view IDs in the helper Android app.
const idPrefix = "org.chromium.arc.testapp.clipboard:id/"

// A copyFunc encapsulates a "copy" operation which is predecided (e.g. data to
// be copied and clipboard destination).
type copyFunc func(context.Context) error

// A copyFunc encapsulates a "paste" operation which is predecided (e.g.
// clipboard source and paste mechanism).
type pasteFunc func(context.Context) (string, error)

// prepareCopyInChrome sets up a copy operation with Chrome as the source
// clipboard.
func prepareCopyInChrome(tconn *chrome.TestConn, format, data string) copyFunc {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, `
		  (format, data) => {
		    document.addEventListener('copy', (event) => {
		      event.clipboardData.setData(format, data);
		      event.preventDefault();
		    }, {once: true});
		    if (!document.execCommand('copy')) {
		      throw new Error('Failed to execute copy');
		    }
		  }`, format, data,
		)
	}
}

// preparePasteInChrome sets up a paste operation with Chrome as the
// destination clipboard.
func preparePasteInChrome(tconn *chrome.TestConn, format string) pasteFunc {
	return func(ctx context.Context) (string, error) {
		var result string
		if err := tconn.Call(ctx, &result, `
		  (format) => {
		    let result;
		    document.addEventListener('paste', (event) => {
		      result = event.clipboardData.getData(format);
		    }, {once: true});
		    if (!document.execCommand('paste')) {
			    throw new Error('Failed to execute paste');
		    }
		    return result;
		  }`, format,
		); err != nil {
			return "", err
		}
		return result, nil
	}
}

// prepareCopyInAndroid sets up a copy operation with Android as the source
// clipboard. The Android helper app we use during this test has a series of
// buttons which populate some views with text (hard-coded into the app); in
// order to execute a copy operation, we first have to click a button (the
// writeDataBtnID) to populate the right text and then we click the
// "copy_button". Since the text to copy is hard-coded into the android app,
// this helper ensures that the app and this test are in-sync with regards to
// the text that we expect to be copied, by checking that the provided
// viewIDForGetText contains the provided expected string.
func prepareCopyInAndroid(d *ui.Device, writeDataBtnID, viewIDForGetText, expected string) copyFunc {
	const copyID = idPrefix + "copy_button"

	return func(ctx context.Context) error {
		if err := d.Object(ui.ID(writeDataBtnID)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to set text in Android EditText")
		}
		if text, err := d.Object(ui.ID(viewIDForGetText)).GetText(ctx); err != nil {
			return errors.Wrap(err, "failed to obtain Android text")
		} else if text != expected {
			return errors.Errorf("failed to set up the content to be copied in Android: got %q; want %q", text, expected)
		}
		if err := d.Object(ui.ID(copyID)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to copy the text in Android")
		}
		return nil
	}
}

// preparePasteInAndroid sets up a paste operation with Android as the
// destination clipboard. Specifically: in the Android helper app, the
// "paste_button" is clicked and the contents of the provided view ID is
// returned via GetText.
func preparePasteInAndroid(d *ui.Device, viewIDForGetText string) pasteFunc {
	const pasteID = idPrefix + "paste_button"

	return func(ctx context.Context) (string, error) {
		if err := d.Object(ui.ID(pasteID)).Click(ctx); err != nil {
			return "", errors.Wrap(err, "failed to paste")
		}

		text, err := d.Object(ui.ID(viewIDForGetText)).GetText(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to obtain pasted text")
		}
		return text, nil
	}
}

func testCopyImageFromChromeToAndroid(ctx context.Context, p *arc.PreData, tconn *chrome.TestConn, fs http.FileSystem) error {
	const (
		apk = "ArcClipboardTest.apk"
		pkg = "org.chromium.arc.testapp.clipboard"
		cls = "org.chromium.arc.testapp.clipboard.ClipboardActivity"

		titleID = idPrefix + "text_view"
		title   = "HTML tags goes here"

		textViewID = idPrefix + "text_view"
	)

	cr := p.Chrome
	a := p.ARC
	d := p.UIDevice

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	server := httptest.NewServer(http.FileServer(fs))
	defer server.Close()

	// Open the html with an image.
	conn, err := cr.NewConn(ctx, server.URL+"/clipboard_image.html")
	if err != nil {
		return errors.Wrap(err, "failed to open clipboard_image.html")
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for the page loaded")
	}

	encodedImage := ""
	conn.Eval(ctx, "document.getElementById('image').getAttribute('src')", &encodedImage)

	if err := uiauto.Combine("copy all text from source website",
		keyboard.AccelAction("Ctrl+A"),
		keyboard.AccelAction("Ctrl+C"))(ctx); err != nil {
		return errors.Wrap(err, "failed to copy text from source browser")
	}

	conn.CloseTarget(ctx)

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		return errors.Wrap(err, "failed to create a new activity")
	}
	defer act.Close()
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the activity")
	}
	defer act.Stop(ctx, tconn)

	if err := d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for the app shown")
	}

	pasteAndroid := preparePasteInAndroid(d, textViewID)
	// Paste in Android.
	androidHTML, err := pasteAndroid(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain pasted image")
	}

	// Verify the result.
	// Note: style attribute is added by Chrome before the image is copied to Android.
	re := regexp.MustCompile(`^<img id="image" src="(.+?)" style=".+?">$`)
	if m := re.FindStringSubmatch(androidHTML); m == nil {
		return errors.Wrapf(err, "failed to find pasted image in Android: got %q", androidHTML)
	} else if m[1] != encodedImage {
		return errors.Wrapf(err, "unexpected paste result: got %q; want %q", m[1], encodedImage)
	}

	return nil
}

func Clipboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcClipboardTest.apk"
		pkg = "org.chromium.arc.testapp.clipboard"
		cls = "org.chromium.arc.testapp.clipboard.ClipboardActivity"

		titleID = idPrefix + "text_view"
		title   = "HTML tags goes here"

		editTextID = idPrefix + "edit_message"
		textViewID = idPrefix + "text_view"
	)

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC
	d := p.UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Copy image from Chrome to Android.
	s.Run(ctx, "CopyImageFromChromeToAndroid", func(ctx context.Context, s *testing.State) {
		if err := testCopyImageFromChromeToAndroid(ctx, p, tconn, s.DataFileSystem()); err != nil {
			s.Fatal("Failed to verify copying an image from a browser to an app: ", err)
		}
	})

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	s.Log("Waiting for App showing up")
	if err := d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for the app shown: ", err)
	}

	info, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		s.Fatal("Failed to window info of the activity: ", err)
	}

	// Click the center of the activity from Chrome to generate the first mouse event, because
	// Wayland's set_selection should be associated with a valid serial number from an actual event.
	if err := mouse.Click(tconn, info.BoundsInRoot.CenterPoint(), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click the center of the app: ", err)
	}

	s.Run(ctx, "CopyHTMLFromChromeToAndroidWithObserver", func(ctx context.Context, s *testing.State) {
		const (
			observerEnableID   = idPrefix + "enable_observer_button"
			observerDisableID  = idPrefix + "disable_observer_button"
			observerTextViewID = idPrefix + "observer_view"
			observerReady      = "Observer ready"
		)
		// Enable observer and wait for it to be ready to prevent a possible race.
		if err := d.Object(ui.ID(observerEnableID)).Click(ctx); err != nil {
			s.Fatal("Failed to enable observer: ", err)
		}
		defer d.Object(ui.ID(observerDisableID)).Click(ctx)
		if err := d.Object(ui.ID(observerTextViewID)).WaitForText(ctx, observerReady, 5*time.Second); err != nil {
			s.Fatal("Failed to wait for observer ready: ", err)
		}

		// Copy in Chrome, so the registered observer should paste the clipboard content in Android.
		const content = "<b>observer</b> should paste this"
		chromeCopy := prepareCopyInChrome(tconn, "text/html", content)
		if err := chromeCopy(ctx); err != nil {
			s.Fatal("Failed to copy in Chrome: ", err)
		}

		// Paste and Verify the result.
		pasteAndroid := preparePasteInAndroid(d, textViewID)
		if html, err := pasteAndroid(ctx); err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		} else if html != content {
			s.Errorf("Failed to copy HTML from Chrome to Android: got %q; want %q", html, content)
		}
	})

	const (
		// clicking writeTextBtnID causes the "Test Text 1234" message to show up
		// in the editTextID.
		writeTextBtnID          = idPrefix + "write_text_button"
		expectedTextFromAndroid = "Test Text 1234"
		// clicking writeHTMLBtnID causes the following HTML to show up in the
		// textViewID.
		writeHTMLBtnID          = idPrefix + "write_html_button"
		expectedHTMLFromAndroid = `<p dir="ltr">test <b>HTML</b> 1234</p>`

		testTextFromChrome = "Text to be copied from Chrome to Android"
		testHTMLFromChrome = "<b>bold</b><i>italics</i>"
	)

	for _, row := range []struct {
		name string

		copyFunc       copyFunc
		pasteFunc      pasteFunc
		wantPastedData string
	}{{
		"CopyTextFromChromeToAndroid",
		prepareCopyInChrome(tconn, "text/plain", testTextFromChrome),
		preparePasteInAndroid(d, editTextID),
		testTextFromChrome,
	}, {
		"CopyTextFromAndroidToChrome",
		prepareCopyInAndroid(d, writeTextBtnID, editTextID, expectedTextFromAndroid),
		preparePasteInChrome(tconn, "text/plain"),
		expectedTextFromAndroid,
	}, {
		"CopyHTMLFromChromeToAndroid",
		prepareCopyInChrome(tconn, "text/plain", testHTMLFromChrome),
		preparePasteInAndroid(d, textViewID),
		testHTMLFromChrome,
	}, {
		"CopyHTMLFromAndroidToChrome",
		prepareCopyInAndroid(d, writeHTMLBtnID, textViewID, expectedHTMLFromAndroid),
		preparePasteInChrome(tconn, "text/html"),
		expectedHTMLFromAndroid,
	}} {
		s.Run(ctx, row.name, func(ctx context.Context, s *testing.State) {
			start := time.Now()
			if err := row.copyFunc(ctx); err != nil {
				s.Fatal("Failed to copy: ", err)
			}

			// Rather than assuming the copy is effective ~immediately, we have to
			// poll because the latency of the operation is slower in newer
			// kernels (e.g. 5.4, at time of writing). See b/157615371 for
			// historical background.
			afterCopy := time.Now()
			attempt := 0
			err := testing.Poll(ctx, func(ctx context.Context) error {
				got, err := row.pasteFunc(ctx)
				if err != nil {
					// We never expect pasting to fail: break from the poll.
					return testing.PollBreak(errors.Wrap(err, "failed to paste"))
				}
				attempt++
				if got == row.wantPastedData {
					msSinceStart := time.Since(start).Seconds() * 1000
					msToPaste := time.Since(afterCopy).Seconds() * 1000
					msToCopy := msSinceStart - msToPaste
					s.Logf("Found expected paste data on attempt #%d; copy took %0.3f ms, paste worked after %0.3f ms", attempt, msToCopy, msToPaste)
					return nil
				}
				return errors.Errorf("after %d paste attempts, the pasted value was %q instead of %q", attempt, got, row.wantPastedData)
			}, &testing.PollOptions{
				// Running the actual paste operation from Chrome to Android can
				// take a long time even for a single iteration (e.g. around 1
				// second), so we are forced to give a relatively high upper bound
				// for the overall timeout.
				Timeout: 5 * time.Second,
				// The latencies we are interested in observing are in the
				// magnitude of tens of milliseconds (for some test cases), so
				// sleep a very short amount of time.
				Interval: 5 * time.Millisecond,
			})
			if err != nil {
				s.Fatal("Failed during paste retry loop: ", err)
			}
		})
	}

	// TODO(ruanc): Copying big text (500Kb) is blocked by https://bugs.chromium.org/p/chromium/issues/detail?id=916882
}
