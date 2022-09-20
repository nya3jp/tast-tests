// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunchOmaha,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests launching and interacting with stateful-lacros across the supported channels served in Omaha",
		Contacts:     []string{"hyungtaekim@chromium.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			// Name:              "omaha",
			// Fixture:           "lacrosOmaha",
			// ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			ExtraAttr: []string{"informational"},
		}},
		Timeout: 4 * time.Minute,
	})
}

// chromeOSChannel returns the channel that the OS image is on. eg. "canary", "dev", "beta", "stable"
func chromeOSChannel() (string, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return "", errors.Wrap(err, "failed to read lsbrelease")
	}

	var channelRe = regexp.MustCompile(`([\w]+)-channel`)
	match := channelRe.FindStringSubmatch(lsb[lsbrelease.ReleaseTrack])
	if match == nil {
		return "", errors.Wrap(err, "failed to read channel info in lsbrelease")
	}
	if match[1] == "testimage" {
		// if testimage, find the channel info in the description key instead of the release track.
		// Example of lsbrelease for testimage:
		//	CHROMEOS_RELEASE_TRACK=testimage-channel
		//	CHROMEOS_RELEASE_DESCRIPTION=15117.0.0 (Official Build) dev-channel atlas test
		match = channelRe.FindStringSubmatch(lsb[lsbrelease.ReleaseDescription])
		if match == nil {
			return "", errors.New("failed to find channel info in lsbrelease")
		}
	}

	channel := match[1]
	// if it is "dev" channel, further check if it is on the "canary" (closer to the ToT) or the "dev" branch by reading the branch number used only by branches.
	if channel == "dev" && lsb[lsbrelease.BranchNumber] == "0" {
		channel = "canary"
	}
	return channel, nil
}

// statefulLacrosChannels resolves the stateful-lacros channels that are within valid version skews to the given OS channel.
// Note that "canary" and "dev" channels are separate
func statefulLacrosChannels(osChannel string) ([]string, error) {
	// TODO(crbug.com/1258138): Update valid version skews when changed. Currently it is [0, +2] of stateful-lacros against OS.
	switch osChannel {
	case "canary":
		return []string{"canary"}, nil
	case "dev":
		return []string{"dev"}, nil
	case "beta":
		return []string{"dev", "beta"}, nil
	case "stable":
		return []string{"dev", "beta", "stable"}, nil
	default:
		return []string{}, errors.Errorf("failed to find valid stateful-lacros channels from OS channel: %v", osChannel)
	}
}

func ensureStatefulLacrosInstalledAtPath(ctx context.Context, tconn *chrome.TestConn) (execPath string, err error) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := lacrosinfo.Snapshot(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get lacros info"))
		}
		if len(info.LacrosPath) == 0 {
			return errors.Wrap(err, "lacros is not yet running (received empty LacrosPath)")
		}
		execPath = filepath.Join(info.LacrosPath, "chrome")
		return nil
	}, nil); err != nil {
		return "", errors.Wrap(err, "lacros is not running")
	}
	return execPath, nil
}

func ShelfLaunchOmaha(ctx context.Context, s *testing.State) {
	osChannel, err := chromeOSChannel()
	if err != nil {
		s.Fatal("Failed to get the OS channel: ", err)
	}
	statefulLacrosChannels, err := statefulLacrosChannels(osChannel)
	if err != nil {
		s.Fatal("Failed to resolve the stateful-lacros channels from the OS channel: ", err)
	}

	// Resolve all compatible stateful-lacros channels from the given OS channel.
	s.Logf("ShelfLaunch with OS: %v, stateful-lacros: %v", osChannel, statefulLacrosChannels)
	cfg := lacrosfixt.NewConfig(lacrosfixt.Selection(lacros.Omaha))
	for _, lacrosChannel := range statefulLacrosChannels {
		opts := []chrome.Option{chrome.ExtraArgs("--lacros-stability=" + lacrosChannel)}

		s.Run(ctx, fmt.Sprintf("OS: %v, stateful-lacros: %v", osChannel, lacrosChannel), func(ctx context.Context, s *testing.State) {
			cr, err := browserfixt.NewChrome(ctx, browser.TypeLacros, cfg, opts...)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to test API: ", err)
			}

			// Ensure Lacros is installed.
			execPath, err := ensureStatefulLacrosInstalledAtPath(ctx, tconn)
			if err != nil {
				// TODO(crbug.com/1367120): Log the stateful-lacros version served in Omaha by the time it fails.
				s.Fatalf("Lacros is not installed from stateful-lacros channel: %v on OS: %v, err: %v", lacrosChannel, osChannel, err)
			}
			s.Log("Lacros is downloaded from Omaha and installed at ", execPath)

			s.Log("Checking that Lacros is included in installed apps")
			appItems, err := ash.ChromeApps(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get installed apps: ", err)
			}
			browser, err := apps.PrimaryBrowser(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get browser app: ", err)
			}
			found := false
			for _, appItem := range appItems {
				if appItem.Type == ash.StandaloneBrowser && appItem.AppID == browser.ID && appItem.Name == browser.Name {
					found = true
					break
				}
			}
			if !found {
				s.Logf("AppID: %v, Name: %v, Type: %v, was expected, but got", browser.ID, browser.Name, ash.StandaloneBrowser)
				for _, appItem := range appItems {
					s.Logf("AppID: %v, Name: %v, Type: %v", appItem.AppID, appItem.Name, appItem.Type)
				}
				s.Fatal("Lacros was not included in the list of installed applications: ", err)
			}

			s.Log("Checking that Lacros is a pinned app in the shelf")
			shelfItems, err := ash.ShelfItems(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get shelf items: ", err)
			}
			found = false
			for _, shelfItem := range shelfItems {
				if shelfItem.AppID == browser.ID && shelfItem.Title == browser.Name && shelfItem.Type == ash.ShelfItemTypePinnedApp {
					found = true
					break
				}
			}
			if !found {
				s.Fatal("Lacros was not found in the list of shelf items: ", err)
			}

			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get all open windows: ", err)
			}
			for _, w := range ws {
				if err := w.CloseWindow(ctx, tconn); err != nil {
					s.Logf("Warning: Failed to close window (%+v): %v", w, err)
				}
			}

			// Clean up user data dir to ensure a clean start.
			os.RemoveAll(lacros.UserDataDir)
			if err = ash.LaunchAppFromShelf(ctx, tconn, browser.Name, browser.ID); err != nil {
				s.Fatal("Failed to launch Lacros: ", err)
			}

			s.Log("Checking that Lacros window is visible")
			if err := lacros.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
				// Grab Lacros logs to assist debugging before exiting.
				lacrosfaillog.Save(ctx, tconn)
				s.Fatal("Failed waiting for Lacros window to be visible: ", err)
			}

			s.Log("Connecting to the lacros-chrome browser")
			l, err := lacros.Connect(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to connect to lacros-chrome: ", err)
			}
			defer func() {
				if l != nil {
					l.Close(ctx)
				}
			}()

			s.Log("Opening a new tab")
			conn, err := l.NewConn(ctx, "about:blank")
			if err != nil {
				s.Fatal("Failed to open new tab: ", err)
			}
			defer conn.Close()
			defer conn.CloseTarget(ctx)
			if err := lacros.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
				s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
			}

			s.Log("Closing lacros-chrome browser")
			if err := l.Close(ctx); err != nil {
				s.Fatal("Failed to close lacros-chrome: ", err)
			}
			l = nil

			if err := ash.WaitForAppClosed(ctx, tconn, browser.ID); err != nil {
				s.Fatalf("%s did not close successfully: %s", browser.Name, err)
			}
		})
	}
}
