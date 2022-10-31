// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EduCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Runs Edu user CUJ in Lacros including installing apps and open webpages with authentication",
		Contacts:     []string{"yjt@google.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosEduGaiaLogin",
		Timeout:      chrome.GAIALoginTimeout + 20*time.Minute,
	})
}

type cujwebpage struct {
	Name string
	URL  string
}

// EduCUJ is defined in go/lacros-audit-cujs. It will install a few extensions, then open a few cujwebpages.
func EduCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer lacrosfaillog.SaveIf(cleanupCtx, tconn, s.HasError)
	// Extensions in cws will be installed
	apps := []cws.App{
		{
			Name: "Read&Write",
			URL:  "https://chrome.google.com/webstore/detail/readwrite-for-google-chro/inoeonmfapjbbkmdafoankkfajkcphgd",
		},
		{
			Name: "Kami",
			URL:  "https://chrome.google.com/webstore/detail/kami-for-google-chrome/ecnphlgnajanjnkcmbpancdjoidceilk",
		},
		{
			Name: "Screencastify",
			URL:  "https://chrome.google.com/webstore/detail/screencastify-screen-vide/mmeijimgabbpbgpdklnllpncmdofkcpn",
		},
	}
	// install the the extensions above
	for _, app := range apps {
		cws.InstallApp(ctx, l.Browser(), tconn, app)
		defer cws.UninstallApp(ctx, l.Browser(), tconn, app)
	}
	// The webpages that will be open in the first window.
	firstWindowCUJWebPages := []cujwebpage{
		{
			Name: "Kids Space",
			URL:  "https://families.google.com/kidsspace/",
		},
		{
			Name: "Youtube",
			URL:  "https://www.youtube.com",
		},
		{
			Name: "Google Slides",
			URL:  "https://slides.google.com",
		},
	}
	// The webpages that will be open in the second window.
	secondWindowCUJWebPages := []cujwebpage{
		{
			Name: "Grow with Google",
			URL:  "https://grow.google/",
		},
	}
	// Start to open the webpages in the first window.
	for _, t := range firstWindowCUJWebPages {
		c, err := l.NewConn(ctx, t.URL)
		if err != nil {
			s.Fatalf("Failed to open Lacros with URL %v: %v", t.URL, err)
		}
		defer c.Close()
	}
	// Open webpages in the second window
	for _, t := range secondWindowCUJWebPages {
		c, err := l.NewConn(ctx, t.URL, browser.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open Lacros with URL %v: %v", t.URL, err)
		}
		defer c.Close()
	}
}
