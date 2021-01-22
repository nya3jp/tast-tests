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

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IntentForward,
		Desc:         "Checks Android intents are forwarded to Chrome",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func IntentForward(ctx context.Context, s *testing.State) {
	const (
		viewAction          = "android.intent.action.VIEW"
		viewDownloadsAction = "android.intent.action.VIEW_DOWNLOADS"
		setWallpaperAction  = "android.intent.action.SET_WALLPAPER"

		filesAppURL        = `chrome-extension://hhaomjibdihmijegdhdafkllkbggdgoj/main.*\.html`
		wallpaperPickerURL = "chrome-extension://obklkkbkpaoaejdabbfldmcfplpdgolj/main.html"
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

	checkIntent := func(action, data, url string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		testing.ContextLogf(ctx, "Testing: %s(%s) -> %s", action, data, url)

		if err := a.SendIntentCommand(ctx, action, data).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to send an intent %q: %v", action, err)
			return
		}

		urlMatcher := func(t *target.Info) bool {
			matched, _ := regexp.MatchString(url, t.URL)
			return matched
		}
		conn, err := cr.NewConnForTarget(ctx, urlMatcher)
		if err != nil {
			s.Errorf("%s(%s) -> %s: %v", action, data, url, err)
		} else {
			conn.Close()
		}
	}

	checkIntent(viewAction, localWebURL, localWebURL)
	checkIntent(setWallpaperAction, "", wallpaperPickerURL)
	if enabled, err := arc.VMEnabled(); err != nil {
		s.Fatal("Failed to check whether ARCVM is enabled: ", err)
	} else if !enabled {
		// ARCVM P does not support launching Files.app from Android.
		// TODO(yusukes): Enable this on ARCVM R.
		checkIntent(viewDownloadsAction, "", filesAppURL)
	}
}
