// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"

	// "chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type clickArea int

const (
	osClickOnly           clickArea = iota // Use LeftClick() to click OS UI (shelf icon and tray)
	browserWindowSwitch                    // use LeftClick() to switch browser windows from shelf
	webClick                               // Use LeftClick() to click web page button
	webNoClick                             // Use keyboard instead of mouse click.
	webClickWithDoDefault                  // use DoDefault() instead of LeftClick()
)

type mouseClickParam struct {
	bt            browser.Type
	clickTestType clickArea
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MouseClick,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test lacros-Chrome metrics and fixtures",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Name:    "lacros_web_click",
				Fixture: "lacrosPrimary",
				Val:     mouseClickParam{browser.TypeLacros, webClick},
			}, {
				Name:    "lacros_web_noclick",
				Fixture: "lacrosPrimary",
				Val:     mouseClickParam{browser.TypeLacros, webNoClick},
			}, {
				Name:    "lacros_web_do_default",
				Fixture: "lacrosPrimary",
				Val:     mouseClickParam{browser.TypeLacros, webClickWithDoDefault},
			}, {
				Name:    "lacros_system_ui_click",
				Fixture: "lacrosPrimary",
				Val:     mouseClickParam{browser.TypeLacros, osClickOnly},
			}, {
				Name:    "lacros_browser_win_switch",
				Fixture: "lacrosPrimary",
				Val:     mouseClickParam{browser.TypeLacros, browserWindowSwitch},
			}, {
				Name:    "ash_web_click",
				Fixture: "chromeLoggedIn",
				Val:     mouseClickParam{browser.TypeAsh, webClick},
			}, {
				Name:    "ash_web_noclick",
				Fixture: "chromeLoggedIn",
				Val:     mouseClickParam{browser.TypeAsh, webNoClick},
			}, {
				Name:    "ash_web_do_default",
				Fixture: "chromeLoggedIn",
				Val:     mouseClickParam{browser.TypeAsh, webClickWithDoDefault},
			}, {
				Name:    "ash_system_ui_click",
				Fixture: "chromeLoggedIn",
				Val:     mouseClickParam{browser.TypeAsh, osClickOnly},
			}, {
				Name:    "ash_browser_win_switch",
				Fixture: "chromeLoggedIn",
				Val:     mouseClickParam{browser.TypeAsh, browserWindowSwitch},
			},
		},
	})
}

func MouseClick(ctx context.Context, s *testing.State) {

	var cr *chrome.Chrome
	var cs ash.ConnSource
	var l *lacros.Lacros
	var err error
	var browserWindowType ash.WindowType

	p := s.Param().(mouseClickParam)

	if p.bt == browser.TypeAsh {
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
		cs = cr
		browserWindowType = ash.WindowTypeBrowser
	} else {
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), p.bt)
		if err != nil {
			s.Fatal("Failed to set up Lacros: ", err)
		}
		browserWindowType = ash.WindowTypeLacros
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	bTconn := tconn
	if l != nil {
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to Lacros test API: ", err)
		}
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.NewPerformanceCUJOptions())
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(ctx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	testing.ContextLog(ctx, "Navigate to google.com")
	_, err = cs.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	// Maximize the browser window, if not yet.
	brw, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.WindowType == browserWindowType && w.State != ash.WindowStateMaximized
	})
	if err != nil {
		s.Fatal("Failed to get browser window: ", err)
	}
	if brw != nil {
		if err := ash.SetWindowStateAndWait(ctx, tconn, brw.ID, ash.WindowStateMaximized); err != nil {
			s.Fatal("Failed to maximize browser window: ", err)
		}
	}

	testing.ContextLog(ctx, "Wait for the UI to be stable before recording")
	testing.Sleep(ctx, 5*time.Second)

	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if p.clickTestType == osClickOnly {
			// Do mouse click on system tray / chrome icon, and return.
			chromeIcon := nodewith.ClassName("ash/ShelfAppButton").NameContaining(chromeApp.Name)
			systemTray := nodewith.HasClass("StatusAreaWidget").Role(role.Window).First()
			return uiauto.NamedCombine("click system tray and chrome icon",
				ui.LeftClick(systemTray),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(systemTray),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(chromeIcon),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(chromeIcon),
				uiauto.Sleep(2*time.Second),
			)(ctx)
		}
		if p.clickTestType == browserWindowSwitch {
			// Create a second browser window.
			_, err = cs.NewConn(ctx, "https://www.google.com", browser.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open new tab: ", err)
			}
			chromeIcon := nodewith.ClassName("ash/ShelfAppButton").NameContaining(chromeApp.Name)
			menuItem1 := nodewith.ClassName("MenuItemView").Nth(1)
			menuItem2 := nodewith.ClassName("MenuItemView").Nth(2)
			return uiauto.NamedCombine("click system tray and chrome icon",
				ui.LeftClick(chromeIcon),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(menuItem1),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(chromeIcon),
				uiauto.Sleep(2*time.Second),
				ui.LeftClick(menuItem2),
				uiauto.Sleep(2*time.Second),
			)(ctx)
		}

		testing.ContextLog(ctx, "Type text in search box")
		if err := kb.Type(ctx, "Hello"); err != nil {
			s.Fatal("Failed to enter search text Hello: ", err)
		}
		testing.ContextLog(ctx, "Type text done. Ready to search")

		testing.Sleep(ctx, 2*time.Second)
		// Press ESC to hide the search box hints.
		if err := kb.AccelAction("ESC")(ctx); err != nil {
			s.Fatal("Failed to press ESC: ", err)
		}
		testing.Sleep(ctx, 3*time.Second)

		if p.clickTestType == webClick || p.clickTestType == webClickWithDoDefault {
			searchBox := nodewith.Name("Google Search").Role(role.Button).First()

			if p.clickTestType == webClick {
				testing.ContextLog(ctx, "Click search button")
				if err := ui.LeftClick(searchBox)(ctx); err != nil {
					s.Fatal("Failed to click search box: ", err)
				}
			} else if p.clickTestType == webClickWithDoDefault {
				testing.ContextLog(ctx, "Do default on search button")
				if err := ui.DoDefault(searchBox)(ctx); err != nil {
					s.Fatal("Failed to doDefault on search button: ", err)
				}
			}
		} else if p.clickTestType == webNoClick {
			if err := kb.AccelAction("Enter")(ctx); err != nil {
				s.Fatal("Failed to typer enter to start search: ", err)
			}
		}

		testing.Sleep(ctx, 5*time.Second)
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test: ", err)
	}

	pv := perf.NewValues()

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}
