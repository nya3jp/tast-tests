// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestLacrosLaunch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A sample to reproduce b/251019896",
		Contacts:     []string{"alfredyu@cienet.com"},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Params: []testing.Param{
			{
				Name: "tconn_from_ash",
				Val:  fromAsh,
			}, {
				Name: "tconn_from_browser",
				Val:  fromBrowser,
			},
		},
	})
}

type tconnType int

const (
	fromAsh tconnType = iota
	fromBrowser
)

func TestLacrosLaunch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection from ash-Chrome: ", err)
	}
	tconnType := s.Param().(tconnType)

	for i := 0; i < 5; i++ {
		s.Log("Starting iteration #", i)
		func() {
			s.Log("Setting up browser")
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to setup browser: ", err)
			}
			defer closeBrowser(ctx)

			if tconnType == fromBrowser {
				if tconn, err = br.TestAPIConn(ctx); err != nil {
					s.Fatal("Failed to get test API connection from browser: ", err)
				}
			}

			s.Log("Closing all tabs")
			if err := browser.CloseAllTabs(ctx, tconn); err != nil {
				s.Log("Failed to close all tabs: ", err)
			}
		}()
	}
}
