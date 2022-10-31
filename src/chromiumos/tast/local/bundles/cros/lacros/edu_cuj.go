// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Contacts:     []string{"yjt@google.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosEduGaiaLogin",
		Timeout:      chrome.GAIALoginTimeout + 20*time.Minute,
	})
}

type webpage struct {
	Name string
	URL  string
}

// EduCUJ is defined in go/lacros-audit-cujs. It will install a few extensions, then open a few webpages.
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
	}
	// wait for the extention installed and stablized.
	testing.Sleep(ctx, 30*time.Second)
	// The webpages that will be open in the first window.
	firstWindowWebPages := []webpage{
		{
			Name: "Wikipedia",
			URL:  "https://www.wikipedia.org",
		},
		{
			Name: "Google Classroom",
			URL:  "https://www.classroom.google.com",
		},
		{
			Name: "Google Docs",
			URL:  "https://www.docs.google.com",
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
	secondWindowWebPages := []webpage{
		{
			Name: "Figma",
			URL:  "https://www.figma.com",
		},
	}
	// Start to open the webpages in the first window.
	var pollOpts = &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}
	for _, t := range firstWindowWebPages {
		// TODO(https://crbug.com/1380087): some webpages requring google account authentication
		// failed to load the first time, it will try a second time.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			c, err := l.NewConn(ctx, t.URL)
			if err != nil {
				return errors.Errorf("failed to open Lacros with URL %v: %v the first time", t.URL, err)
			}
			defer c.Close()
			return nil
		}, pollOpts); err != nil {
			c, err := l.NewConn(ctx, t.URL)
			if err != nil {
				s.Fatalf("Failed to open Lacros with URL %v: %v the second time", t.URL, err)
			}
			defer c.Close()
		}
	}
	// Open webpages in the second window
	for _, t := range secondWindowWebPages {
		c, err := l.NewConn(ctx, t.URL, browser.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open Lacros with URL %v: %v", t.URL, err)
		}
		defer c.Close()
	}
	// Uninstall the extensions from Lacros.
	for _, app := range apps {
		cws.UninstallApp(ctx, l.Browser(), tconn, app)
		// When uninstalling the extension, some app will open new feedback page, which
		// takes time to load.
		testing.Sleep(ctx, time.Second*20)
	}
	defer lacrosfaillog.SaveIf(cleanupCtx, tconn, s.HasError)
}
