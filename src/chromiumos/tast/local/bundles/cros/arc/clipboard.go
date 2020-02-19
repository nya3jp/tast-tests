// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Clipboard,
		Desc:         "Tests copying and pasting from Chrome to Android and vice versa",
		Contacts:     []string{"ruanc@chromium.org", "yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcClipboardTest.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
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

		expectedTextFromAndroid = "Test Text 1234"
		expectedHTMLFromAndroid = `<p dir="ltr">test <b>HTML</b> 1234</p>`
	)

	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	s.Log("Starting app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Waiting for App showing up")
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	if err := d.Object(ui.ID(titleID), ui.Text(title)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for the app shown: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test connection: ", err)
	}
	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	copyInChrome := func(format, data string) error {
		return tconn.Exec(ctx, fmt.Sprintf(`
                  (function() {
                    document.addEventListener('copy', (event) => {
                      event.clipboardData.setData(%q, %q);
                      event.preventDefault();
                    }, {once: true});
                    if (!document.execCommand('copy')) {
                      throw new Error('Failed to execute copy');
                    }
                  })()
                `, format, data))
	}

	pasteInChrome := func(format string) (string, error) {
		var result string
		if err := tconn.Eval(ctx, fmt.Sprintf(`
                  (function() {
                    let result;
                    document.addEventListener('paste', (event) => {
                      result = event.clipboardData.getData(%q);
                    }, {once: true});
                    if (!document.execCommand('paste')) {
                      throw new Error('Failed to execute paste');
                    }
                    return result;
                  })()
                `, format), &result); err != nil {
			return "", err
		}
		return result, nil
	}

	s.Run(ctx, "CopyTextFromChromeToAndroid", func(ctx context.Context, s *testing.State) {
		// Copy in Chrome.
		const content = "Text to be copied from Chrome to Android"
		if err := copyInChrome("text/plain", content); err != nil {
			s.Fatal("Failed to copy text: ", err)
		}

		// Paste in Android.
		if err := d.Object(ui.ID(pasteID)).Click(ctx); err != nil {
			s.Fatal("Failed to paste: ", err)
		}

		// Verify the result.
		if text, err := d.Object(ui.ID(editTextID)).GetText(ctx); err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		} else if text != content {
			s.Errorf("Failed to copy text from Chrome to Android: got %q; want %q", text, content)
		}
	})

	s.Run(ctx, "CopyTextFromAndroidToChrome", func(ctx context.Context, s *testing.State) {
		// Set up text to be copied in Android.
		const expected = "Test Text 1234"
		if err := d.Object(ui.ID(writeTextBtnID)).Click(ctx); err != nil {
			s.Fatal("Failed to set text in Android EditText: ", err)
		}
		if text, err := d.Object(ui.ID(editTextID)).GetText(ctx); err != nil {
			s.Fatal("Failed to obtain Anroid text: ", err)
		} else if text != expected {
			s.Fatalf("Failed to set up the content to be copied in Android: got %q; want %q", text, expected)
		}

		// Copy in Android.
		if err := d.Object(ui.ID(copyID)).Click(ctx); err != nil {
			s.Fatal("Failed to copy the text in Android: ", err)
		}

		// Paste in Chrome.
		if text, err := pasteInChrome("text/plain"); err != nil {
			s.Fatal("Failed to execute paste in Chrome: ", err)
		} else if text != expected {
			s.Errorf("Failed to copy text from Android to Chrome: got %q; want %q", text, expected)
		}
	})

	s.Run(ctx, "CopyHTMLFromChromeToAndroid", func(ctx context.Context, s *testing.State) {
		// Copy in Chrome.
		const content = "<b>bold</b><i>italics</i>"
		if err := copyInChrome("text/html", content); err != nil {
			s.Fatal("Failed to copy HTML: ", err)
		}

		// Paste in Android.
		if err := d.Object(ui.ID(pasteID)).Click(ctx); err != nil {
			s.Fatal("Failed to paste: ", err)
		}

		// Verify the result.
		if html, err := d.Object(ui.ID(textViewID)).GetText(ctx); err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		} else if html != content {
			s.Errorf("Failed to copy HTML from Chrome to Android: got %q; want %q", html, content)
		}
	})

	s.Run(ctx, "CopyHTMLFromAndroidToChrome", func(ctx context.Context, s *testing.State) {
		// Set up HTML to be copied in Android.
		const expected = `<p dir="ltr">test <b>HTML</b> 1234</p>`
		if err := d.Object(ui.ID(writeHTMLBtnID)).Click(ctx); err != nil {
			s.Fatal("Failed to set HTML in Android EditText: ", err)
		}
		if html, err := d.Object(ui.ID(textViewID)).GetText(ctx); err != nil {
			s.Fatal("Failed to obtain Anroid HTML: ", err)
		} else if html != expected {
			s.Fatalf("Failed to set up HTML to be copied in Android: got %q; want %q", html, expected)
		}

		// Copy in Android.
		if err := d.Object(ui.ID(copyID)).Click(ctx); err != nil {
			s.Fatal("Failed to copy the HTML in Android: ", err)
		}

		// Paste in Chrome.
		if html, err := pasteInChrome("text/html"); err != nil {
			s.Fatal("Failed to execute paste in Android: ", err)
		} else if html != expected {
			s.Errorf("Failed to copy HTML from Android to Chrome: got %q; want %q", html, expected)
		}
	})

	// Copy image from Chrome to Android.
	s.Run(ctx, "CopyImageFromChromeToAndroid", func(ctx context.Context, s *testing.State) {
		const encodedImage = "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"

		// Copy image in Chrome. This creates a temporary dom to be copied,
		// and destroys it after the copy operation.
		if err := tconn.Exec(ctx, fmt.Sprintf(`
                  (function() {
                    const img = document.createElement('img');
                    img.src = 'data:image/png;base64,' + %q;
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
                  })()
                `, encodedImage)); err != nil {
			s.Fatal("Failed to copy image in Chrome: ", err)
		}

		// Paste in Android.
		if err := d.Object(ui.ID(pasteID)).Click(ctx); err != nil {
			s.Fatal("Failed to paste: ", err)
		}

		// Verify the result.
		androidHTML, err := d.Object(ui.ID(textViewID)).GetText(ctx)
		if err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		}

		// Note: style attribute is added by Chrome before the image is copied to Android.
		re := regexp.MustCompile(`^<img src="data:image/png;base64,(.+?)" style=".+?">$`)
		if m := re.FindStringSubmatch(androidHTML); m == nil {
			s.Fatalf("Failed to find pasted image in Android: got %q", androidHTML)
		} else if m[1] != encodedImage {
			s.Fatalf("Unexpected paste result: got %q; want %q", m[1], encodedImage)
		}
	})

	s.Run(ctx, "CopyHTMLFromChromeToAndroidWithObserver", func(ctx context.Context, s *testing.State) {
		// Enable observer and wait for it to be ready to prevent a possible race.
		if err := d.Object(ui.ID(observerEnableID)).Click(ctx); err != nil {
			s.Fatal("Failed to enable observer: ", err)
		}
		defer d.Object(ui.ID(observerDisableID)).Click(ctx)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if msg, err := d.Object(ui.ID(observerTextViewID)).GetText(ctx); err != nil {
				return testing.PollBreak(err)
			} else if msg != observerReady {
				return errors.New("observer is not yet ready")
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed to wait for observer ready: ", err)
		}

		// Copy in Chrome, so the registered observer should paste the clipboard content in Android.
		const content = "<b>observer</b> should paste this"
		if err := copyInChrome("text/html", content); err != nil {
			s.Fatal("Failed to copy in Chrome: ", err)
		}

		// Verify the result.
		if html, err := d.Object(ui.ID(textViewID)).GetText(ctx); err != nil {
			s.Fatal("Failed to obtain pasted text: ", err)
		} else if html != content {
			s.Errorf("Failed to copy HTML from Chrome to Android: got %q; want %q", html, content)
		}
	})

	// TODO(ruanc): Copying big text (500Kb) is blocked by https://bugs.chromium.org/p/chromium/issues/detail?id=916882
}
