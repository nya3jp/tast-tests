// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IntentForward,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Android intents are forwarded to Chrome",
		Contacts:     []string{"djacobo@google.com", "arc-core@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Attr:         []string{"group:mainline", "group:arc-functional"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               browser.TypeAsh,
			Fixture:           "arcBooted",
		}, {
			Name: "lacros",
			// TODO(b/239469085): Remove "informational" attribute.
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Val:               browser.TypeLacros,
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               browser.TypeAsh,
			Fixture:           "arcBooted",
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Val:               browser.TypeLacros,
			Fixture:           "lacrosWithArcBooted",
		}},
	})
}

func IntentForward(ctx context.Context, s *testing.State) {
	const (
		viewAction          = "android.intent.action.VIEW"
		viewDownloadsAction = "android.intent.action.VIEW_DOWNLOADS"
		setWallpaperAction  = "android.intent.action.SET_WALLPAPER"

		filesAppURL        = `chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.*\.html`
		wallpaperPickerURL = "chrome://personalization/wallpaper"
	)

	d := s.FixtValue().(*arc.PreData)
	a := d.ARC
	cr := d.Chrome

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "It worked!")
	}))
	defer server.Close()
	localWebURL := server.URL + "/" // Must end with a slash

	checkIntent := func(action, data, url string, bt browser.Type) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		testing.ContextLogf(ctx, "Testing: %s(%s) -> %s", action, data, url)

		if err := a.SendIntentCommand(ctx, action, data).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to send an intent %q: %v", action, err)
			return
		}

		br, brCleanUp, err := browserfixt.Connect(ctx, cr, bt)
		if err != nil {
			s.Error(err, "failed to connect to browser")
		}
		defer brCleanUp(ctx)
		urlMatcher := func(t *target.Info) bool {
			matched, _ := regexp.MatchString(url, t.URL)
			return matched
		}

		conn, err := br.NewConnForTarget(ctx, urlMatcher)
		if err != nil {
			s.Errorf("%s(%s) -> %s: %v", action, data, url, err)
		}
		defer conn.Close()
	}

	checkIntent(viewAction, localWebURL, localWebURL, s.Param().(browser.Type))
	checkIntent(setWallpaperAction, "", wallpaperPickerURL, browser.TypeAsh)
	checkIntent(viewDownloadsAction, "", filesAppURL, browser.TypeAsh)
}
