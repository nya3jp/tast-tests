// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScrollToTextFragmentEnabled,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that the ScrollToTextFragmentEnabled policy is correctly applied",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func pageWithTextFragment(textFragment string) string {
	page := `<!doctype html>
<html lang="en">

<head>
  <meta charset="utf-8">
  <title>Scroll to Text Fragment Test Page</title>
</head>
<body>`

	// Add 500 lines so page has to scroll.
	for i := 0; i < 500; i++ {
		page += fmt.Sprintf("<div>%d</div>", i)
	}

	page += fmt.Sprintf("<div id=\"fragment\">%s</div>", textFragment)

	page += `</body>
</html>`

	return page
}

func ScrollToTextFragmentEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	textFragment := "loremipsum"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, pageWithTextFragment(textFragment))
	}))
	defer server.Close()

	for _, param := range []struct {
		name  string
		value *policy.ScrollToTextFragmentEnabled
	}{
		{
			name:  "enabled",
			value: &policy.ScrollToTextFragmentEnabled{Val: true},
		},
		{
			name:  "disabled",
			value: &policy.ScrollToTextFragmentEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.ScrollToTextFragmentEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open lacros browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Open page with text fragment identifier.
			url := server.URL + "#:~:text=" + textFragment
			if err := conn.Navigate(ctx, url); err != nil {
				s.Fatalf("Failed to navigate to the server URL %q: %v", server.URL, err)
			}
			defer conn.Close()

			// We can't test if the text fragment is highlighted as the highlighting is not part of the accessbility
			// tree. Check that the text fragment has scrolled into view instead.
			inView := isInView(ctx, s, conn, "fragment")
			if param.value.Val || param.value.Stat == policy.StatusUnset {
				// Policy is enabled or unset, should have scrolled to text fragment.
				if !inView {
					s.Fatal("Text fragment unexpectedly not in view")
				}
			} else {
				// Policy is disabled, should not have scrolled to text fragment.
				if inView {
					s.Fatal("Text fragment unexpectedly in view")
				}
			}
		})
	}
}

// isInView checks if the element with fragmentID is in the viewport of the window by using an IntersectionObserver.
func isInView(ctx context.Context, s *testing.State, conn *browser.Conn, fragmentID string) bool {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	inView := false
	if err := conn.Call(ctx, &inView, `(id) => {
		let options = {root: null, threshold: 1.0};
		let target = document.querySelector('#' + id);

		return new Promise((resolve) => {
			let observer = new IntersectionObserver((entries) => {
				if (entries.length == 0) {
					reject('Could not find element with id ' + id)
					return;
				}

				resolve(entries[0].isIntersecting)
			}, options);

			observer.observe(target);
		});
	}`, fragmentID); err != nil {
		s.Fatal("Could not check if fragment was in view: ", err)
	}

	return inView
}
