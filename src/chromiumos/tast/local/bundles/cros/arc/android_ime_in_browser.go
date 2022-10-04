// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndroidIMEInBrowser,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Android IME in a browser window",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBootedInTabletMode",
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBootedInTabletMode",
			Val:               browser.TypeLacros,
		}},
	})
}

func getThirdPartyInputMethodID(ctx context.Context, tconn *chrome.TestConn, pkg string) (string, error) {
	imes, err := ime.BrowserInputMethodLists(ctx, tconn)
	if err != nil {
		return "", err
	}
	for _, im := range imes.ThirdPartyExtensionIMEs {
		if strings.Contains(im.ID, pkg) {
			return im.ID, nil
		}
	}
	return "", errors.Errorf("%s not found in the list", pkg)
}

func AndroidIMEInBrowser(ctx context.Context, s *testing.State) {
	const (
		apk         = "ArcInputMethodTest.apk"
		pkg         = "org.chromium.arc.testapp.ime"
		settingsPkg = "com.android.settings"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC
	dev := p.UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	s.Log("Installing IME service")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
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
	if err := ime.AddInputMethod(ctx, tconn, id); err != nil {
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
	if err := ime.SetCurrentInputMethod(ctx, tconn, id); err != nil {
		s.Fatal("Failed to activate ARC Test IME: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		actualID, err := ime.CurrentInputMethod(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
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
	const html = `<input type="text" class="text" autocorrect="off" autofocus/>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer for test page failed: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for the page loaded: ", err)
	}

	s.Log("Waiting for the text field to focus")
	uia := uiauto.New(tconn)
	finder := nodewith.Role(role.TextField).HasClass("text").First()
	if err := uia.EnsureFocused(finder)(ctx); err != nil {
		s.Fatal("Failed to wait for text field to focus: ", err)
	}

	s.Log("Showing the virtual keyboard")
	keyboard := nodewith.ClassName("ExoInputMethodSurface")
	showVirtualKeyboardIfEnabled := func(ctx context.Context) error {
		// Repeatedly call showVirtualKeyboardIfEnabled until isKeyboardShown returns true.
		// Usually it requires only one call of showVirtualKeyboardIfEnabled, but on rare occasions it requires
		// multiple ones e.g. the function was called right in the middle of IME switch.
		if err := tconn.Eval(ctx, `chrome.autotestPrivate.showVirtualKeyboardIfEnabled()`, nil); err != nil {
			return errors.New("virtual keyboard still not shown")
		}
		return nil
	}

	if err := uia.RetryUntil(showVirtualKeyboardIfEnabled, uia.Exists(keyboard))(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	// You can only type "a" from the test IME.
	const expected = "aaaa"

	s.Log("Trying to press the button in ARC Test IME")
	// It is better to press the keyboard from chromeui instead of UIAutomator for two reasons:
	// - IME is not accessible from UIAutomator, so we're forced to use UiDevice.click.
	// - chromeui can provide test coverage for window input region.
	if err := uiauto.Repeat(len(expected), uia.LeftClick(keyboard))(ctx); err != nil {
		s.Fatal("Failed to left click the keyboard")
	}

	s.Log("Waiting for the text field to have the correct contents")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		nodeInfo, err := uia.Info(ctx, finder)
		if err != nil {
			return err
		}
		if nodeInfo.Value != expected {
			return errors.Errorf("got input %q from field after typing %q", nodeInfo.Value, expected)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Error("Failed to get input text: ", err)
	}

	// Hide the keyboard using the "back" button in shelf.
	if err := uia.LeftClick(nodewith.ClassName("ash/BackButton"))(ctx); err != nil {
		s.Fatal("Failed to click shelf back button: ", err)
	}

	if err := uia.WaitUntilGone(keyboard)(ctx); err != nil {
		s.Fatal("Keyboard not hidden after clicking back button")
	}

}
