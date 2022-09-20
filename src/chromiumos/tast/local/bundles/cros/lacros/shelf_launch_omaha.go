// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunchOmaha,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests launching and interacting with stateful-lacros across the supported channels served in Omaha",
		Contacts:     []string{"hyungtaekim@chromium.org", "chromeos-sw-engprod@google.com", "lacros-tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		HardwareDeps: hwdep.D(hwdep.Model( // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			"kasumi", "vilboz" /* amd64 */, "krane" /* arm */)),
		Timeout: 4 * time.Minute,
	})
}

// chromeOSChannel returns the channel that the OS image is released from. eg. "canary", "dev", "beta", "stable"
// It reads lsb-release for the release channel info.
// By the way, sometimes (when a new branch is not yet cut for a new milestone) both "canary" and "dev" are from trunk not in separate branches.
// If so, the branch number will be used to distinguish between "canary" (with the number 0) and "dev" channel (>0).
func chromeOSChannel() (string, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return "", errors.Wrap(err, "failed to read lsbrelease")
	}

	var channelRe = regexp.MustCompile(`(dev|beta|stable|testimage)-channel`)
	match := channelRe.FindStringSubmatch(lsb[lsbrelease.ReleaseTrack])
	if match == nil {
		return "", errors.Wrapf(err, "failed to read channel in lsbrelease's releasetrack: %v", lsb[lsbrelease.ReleaseTrack])
	}
	if match[1] == "testimage" {
		// if testimage, find the channel info in the description key instead of the release track.
		// Example of the testimage's lsbrelease:
		//	CHROMEOS_RELEASE_TRACK=testimage-channel
		//	CHROMEOS_RELEASE_DESCRIPTION=15117.0.0 (Official Build) dev-channel atlas test
		match = channelRe.FindStringSubmatch(lsb[lsbrelease.ReleaseDescription])
		if match == nil {
			return "", errors.New("failed to read channel in lsbrelease's description")
		}
	}

	osChannel := match[1]
	// if it is "dev" channel, further check if it is on "canary" pre-branch or "dev" post-branch by checking the branch number. eg, "canary" with branch number == 0.
	if osChannel == "dev" && lsb[lsbrelease.BranchNumber] == "0" {
		osChannel = "canary"
	}
	return osChannel, nil
}

// statefulLacrosChannels resolves the stateful-lacros channels from the given OS channel within the supported version skews.
// This includes public OS user channels (eg, "dev", "beta" and "stable") and also a special channel "canary" for extra test coverage on trunk.
func statefulLacrosChannels(osChannel string) ([]string, error) {
	// TODO(crbug.com/1258138): Update valid version skews when changed. Currently it is [0, +2] of stateful-lacros against OS.
	switch osChannel {
	case "canary":
		return []string{"canary"}, nil
	case "dev":
		return []string{"dev"}, nil
	case "beta":
		return []string{"beta", "dev"}, nil
	case "stable":
		return []string{"stable", "beta", "dev"}, nil
	default:
		return []string{}, errors.Errorf("failed to find valid stateful-lacros channels from OS channel: %v", osChannel)
	}
}

func waitForLacrosPath(ctx context.Context, tconn *chrome.TestConn) (execPath string, err error) {
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
	}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: time.Second}); err != nil {
		return "", errors.Wrap(err, "lacros is not running")
	}
	return execPath, nil
}

func clearStatefulLacros(ctx context.Context) {
	// Mark the stateful partition corrupt, so the provision can restore it.
	// Remove it only if the clean up is successful.
	if err := ioutil.WriteFile(lacroscommon.CorruptStatefulFilePath, nil, 0644); err != nil {
		testing.ContextLog(ctx, "Failed to touch file: ", err)
	}

	// Try to unmount provisioned stateful-lacros, then remove mount points.
	matches, _ := filepath.Glob("/run/imageloader/lacros*/*")
	for _, match := range matches {
		if err := testexec.CommandContext(ctx, "umount", "-f", match).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unmount ", match)
		}
		if err := os.RemoveAll(match); err != nil {
			testing.ContextLog(ctx, "Failed to remove ", match)
		}
	}

	// Remove provisioned files. Note that 'sh' is used to handle the glob.
	lacrosComponentPathGlob := filepath.Join(lacroscommon.LacrosRootComponentPath, "*")
	if err := testexec.CommandContext(ctx, "sh", "-c",
		strings.Join([]string{"rm", "-rf", lacrosComponentPathGlob}, " ")).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to remove provisioned components at ", lacrosComponentPathGlob)
	}

	// If succeeded to clear, we no longer need to mark the stateful partition corrupt.
	matches, _ = filepath.Glob(lacrosComponentPathGlob)
	if len(matches) == 0 {
		os.Remove(lacroscommon.CorruptStatefulFilePath)
	}
}

func ShelfLaunchOmaha(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Read the OS channel and stateful-lacros channels that are compatible with each other.
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
		s.Run(ctx, fmt.Sprintf("OS: %v, stateful-lacros: %v", osChannel, lacrosChannel), func(ctx context.Context, s *testing.State) {
			cr, err := browserfixt.NewChrome(ctx, browser.TypeLacros, cfg,
				chrome.ExtraArgs("--lacros-stability="+lacrosChannel))
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to test API: ", err)
			}

			// Ensure Lacros is installed.
			execPath, err := waitForLacrosPath(ctx, tconn)
			if err != nil {
				// TODO(crbug.com/1367120): Log the stateful-lacros version served in Omaha by the time it fails.
				s.Fatalf("Lacros is not installed from stateful-lacros channel: %v on OS: %v, err: %v", lacrosChannel, osChannel, err)
			}
			s.Log("Lacros is installed at ", execPath)

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

			// Reset Lacros to purge user data and close open windows for a clean start.
			if err := lacros.ResetState(ctx, tconn); err != nil {
				s.Fatal("Failed resetting Lacros state: ", err)
			}
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
				s.Log("Closing lacros-chrome browser")
				if err := l.Close(ctx); err != nil {
					s.Fatal("Failed to close lacros-chrome: ", err)
				}
				if err := ash.WaitForAppClosed(ctx, tconn, browser.ID); err != nil {
					s.Fatalf("%s did not close successfully: %s", browser.Name, err)
				}
			}()

			s.Log("Opening a new blank page")
			conn, err := l.NewConn(ctx, chrome.BlankURL)
			if err != nil {
				s.Fatal("Failed to open new tab: ", err)
			}
			defer conn.Close()
			defer conn.CloseTarget(cleanupCtx)
			if err := lacros.WaitForLacrosWindow(ctx, tconn, chrome.BlankURL); err != nil {
				s.Fatal("Failed waiting for Lacros to open new tab page: ", err)
			}
		})
	}

	s.Log("Cleaning up stateful partition for subsequent tests")
	clearStatefulLacros(cleanupCtx)
}
