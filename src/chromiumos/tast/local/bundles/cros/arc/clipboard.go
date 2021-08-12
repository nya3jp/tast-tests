// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Clipboard,
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa",
		Contacts:     []string{"ruanc@chromium.org", "yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Starting app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	s.Log("Waiting for App showing up")
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)
	if err := d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for the app shown: ", err)
	}

	rect, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the window bounds: ", err)
	}

	// Click the center of the activity from Chrome to generate the first mouse event, because
	// Wayland's set_selection should be associated with a valid serial number from an actual event.
	if err := mouse.Click(ctx, tconn, rect.CenterPoint(), mouse.LeftButton); err != nil {
		s.Fatal("Failed to click the center of the app: ", err)
	}

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	// Copy image from Chrome to Android.
	s.Run(ctx, "CopyImageFromChromeToAndroid", func(ctx context.Context, s *testing.State) {
		const encodedImage = "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"

		// Copy image in Chrome. This creates a temporary dom to be copied,
		// and destroys it after the copy operation.
		if err := tconn.Call(ctx, nil, `(encodedImage) => {
                  const img = document.createElement('img');
                  img.src = 'data:image/png;base64,' + encodedImage;
                  const container = document.createElement('div');
                  container.appendChild(img);
                  document.body.appendChild(container);
                  try {
                    const range = document.createRange();
                    range.selectNodeContents(container);
                    const selection = window.getSelection();
                    selection.removeAllRanges();
                    selection.addRange(range);
                    return document.execCommand('copy');
                  } finally {
                    document.body.removeChild(container);
                  }
                }`, encodedImage); err != nil {
			s.Fatal("Failed to copy image in Chrome: ", err)
		}

		pasteAndroid := preparePasteInAndroid(d, textViewID)
		// Paste in Android.
		androidHTML, err := pasteAndroid(ctx)
		if err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		}

		// Verify the result.
		// Note: style attribute is added by Chrome before the image is copied to Android.
		re := regexp.MustCompile(`^<img src="data:image/png;base64,(.+?)" style=".+?">$`)
		if m := re.FindStringSubmatch(androidHTML); m == nil {
			s.Fatalf("Failed to find pasted image in Android: got %q", androidHTML)
		} else if m[1] != encodedImage {
			s.Fatalf("Unexpected paste result: got %q; want %q", m[1], encodedImage)
		}
	})

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
