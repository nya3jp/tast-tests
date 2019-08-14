// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type installationType int

const (
	system installationType = iota
	local
)

type config struct {
	desktopFile string
	iconFile    string
	windowName  string
	installType installationType
	// TODO(hollingum): This field is redundant. Add an autotest api that
	// gets the value computed from hashing vm, container and desktop file.
	launcherID string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Launcher,
		Desc:     "Runs applications from the launcher in low/high-DPI mode",
		Contacts: []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:     []string{"informational"},
		Params: []testing.Param{{
			Name:      "local_wayland",
			ExtraData: []string{"launcher_wayland_demo_fixed_size.desktop", "launcher_wayland_demo.png"},
			Val: config{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installType: local,
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			}}, {
			Name:      "local_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: config{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installType: local,
				launcherID:  "mddfmcdnhpnhoefmmiochnnjofmfhanb",
			}}, {
			Name:      "system_wayland",
			ExtraData: []string{"launcher_wayland_demo_fixed_size.desktop", "launcher_wayland_demo.png"},
			Val: config{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installType: system,
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			}}, {
			Name:      "system_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: config{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installType: system,
				launcherID:  "mddfmcdnhpnhoefmmiochnnjofmfhanb",
			},
		}},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func Launcher(ctx context.Context, s *testing.State) {
	conf := s.Param().(config)
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	cont := pre.Container
	ownerID := cont.VM.Concierge.GetOwnerID()

	// Confirm we don't have the application going-in or leaving.
	checkIconExistence(ctx, s, ownerID, conf.launcherID, false)
	defer checkIconExistence(ctx, s, ownerID, conf.launcherID, false)

	if conf.installType == local {
		iconPath := "/home/testuser/.local/share/icons/hicolor/32x32/apps/" + conf.iconFile
		desktopPath := "/home/testuser/.local/share/applications/" + conf.desktopFile
		crostini.TransferToContainerOrDie(ctx, s, cont, s.DataPath("launcher_"+conf.iconFile), iconPath)
		defer crostini.RemoveContainerFile(ctx, cont, iconPath)
		crostini.TransferToContainerOrDie(ctx, s, cont, s.DataPath("launcher_"+conf.desktopFile), desktopPath)
		defer crostini.RemoveContainerFile(ctx, cont, desktopPath)
	} else {
		iconPath := "/usr/share/icons/hicolor/32x32/apps/" + conf.iconFile
		desktopPath := "/usr/share/applications/" + conf.desktopFile
		crostini.TransferToContainerAsRootOrDie(ctx, s, cont, s.DataPath("launcher_"+conf.iconFile), iconPath)
		defer crostini.RemoveContainerFile(ctx, cont, iconPath)
		crostini.TransferToContainerAsRootOrDie(ctx, s, cont, s.DataPath("launcher_"+conf.desktopFile), desktopPath)
		defer crostini.RemoveContainerFile(ctx, cont, desktopPath)
	}

	// There's a delay with apps being installed in Crostini and them appearing
	// in the launcher as well as having their icons loaded. The icons are only
	// loaded after they appear in the launcher, so if we check that first we know
	// it is in the launcher afterwards.
	checkIconExistence(ctx, s, ownerID, conf.launcherID, true)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	sizeHighDensity, err := launchAppAndMeasureWindowSize(ctx, s, tconn, keyboard, ownerID, conf.launcherID, conf.windowName, false)
	if err != nil {
		s.Fatal("Failed getting high density window size: ", err)
	}
	sizeLowDensity, err := launchAppAndMeasureWindowSize(ctx, s, tconn, keyboard, ownerID, conf.launcherID, conf.windowName, true)
	if err != nil {
		s.Fatal("Failed getting low density window size: ", err)
	}

	if err := crostini.VerifyWindowDensities(ctx, tconn, sizeHighDensity, sizeLowDensity); err != nil {
		s.Fatal("Failed during window density comparison: ", err)
	}

}

// launchAppAndMeasureWindowSize is a helper function that sets the app "scaled" property, launches the app and returns its window size.
func launchAppAndMeasureWindowSize(ctx context.Context, s *testing.State, tconn *chrome.Conn,
	ew *input.KeyboardEventWriter, ownerID, appID string, windowName string, scaled bool) (crostini.Size, error) {
	s.Log("Setting application property 'scaled' to ", scaled)
	if err := setAppScaled(ctx, s, tconn, appID, scaled); err != nil {
		return crostini.Size{}, err
	}

	launchApplication(ctx, s, tconn, appID)

	sz, err := crostini.PollWindowSize(ctx, tconn, windowName)
	if err != nil {
		return crostini.Size{}, err
	}
	s.Log("Window size is ", sz)

	if visible, err := getShelfVisibility(ctx, s, tconn, appID); err != nil {
		return crostini.Size{}, err
	} else if !visible {
		return crostini.Size{}, errors.New("App was not visible in shelf after opening")
	}

	// Close the application with a keypress.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	// This may not happen instantaneously, so poll for it.
	checkVisible := func(ctx context.Context) error {
		if visible, err := getShelfVisibility(ctx, s, tconn, appID); err != nil {
			return err
		} else if visible {
			return errors.New("app was visible in shelf after closing")
		}
		return nil
	}
	if err := testing.Poll(ctx, checkVisible, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return crostini.Size{}, err
	}
	return sz, nil
}

// checkIconExistence verifies that the Crostini icon folder for the specified
// application exists (or doesnt) in the filesystem and contains at least one file.
func checkIconExistence(ctx context.Context, s *testing.State, ownerID, appID string, expectExists bool) {
	iconDir := filepath.Join("/home/user", ownerID, "crostini.icons", appID)
	errorIfExpectIs := func(existenceCase bool, err error) error {
		if existenceCase == expectExists {
			return err
		}
		return nil
	}
	existenceCheck := func(ctx context.Context) error {
		fileInfo, err := os.Stat(iconDir)
		if err != nil {
			return errorIfExpectIs(true, err)
		}
		if !fileInfo.IsDir() {
			return errorIfExpectIs(true, errors.Errorf("icon path %v is not a directory", iconDir))
		}
		entries, err := ioutil.ReadDir(iconDir)
		if err != nil {
			return errors.Wrapf(err, "failed reading dir %v", iconDir)
		}
		if len(entries) == 0 {
			return errorIfExpectIs(true, errors.Errorf("no icons exist in %v", iconDir))
		}
		return errorIfExpectIs(false, errors.Errorf("icons exist in %v", iconDir))
	}
	if err := testing.Poll(ctx, existenceCheck, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		s.Errorf("Failed checking icons exist in %q (where expectExists is %v): %v", iconDir, expectExists, err)
	} else {
		s.Logf("Icon for %q passed existence check (where expectExists is %v)", appID, expectExists)
	}
}

// launchApplication launches the specified application via an autotest API call.
func launchApplication(ctx context.Context, s *testing.State, tconn *chrome.Conn, appID string) error {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.launchApp('%v', () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appID)
	return tconn.EvalPromise(ctx, expr, nil)
}

// setAppScaled sets the specified application to be scaled or not via an autotest API call.
func setAppScaled(ctx context.Context, s *testing.State, tconn *chrome.Conn, appID string, scaled bool) error {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.setCrostiniAppScaled('%v', %v, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appID, scaled)
	return tconn.EvalPromise(ctx, expr, nil)
}

// getShelfVisibility makes an autotest API call to determine if the specified
// application has a shelf icon that is in the running state and returns true
// if so, false otherwise.
func getShelfVisibility(ctx context.Context, s *testing.State, tconn *chrome.Conn, appID string) (bool, error) {
	var appShown bool
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.isAppShown('%v', function(appShown) {
				if (chrome.runtime.lastError === undefined) {
					resolve(appShown);
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appID)
	if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
		return false, err
	}
	return appShown, nil
}
