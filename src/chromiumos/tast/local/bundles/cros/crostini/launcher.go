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

type launcherTestConfig struct {
	desktopFile string
	iconFile    string
	windowName  string
	installRoot string
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
			Val: launcherTestConfig{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installRoot: "/home/testuser/.local",
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			},
		}, {
			Name:      "local_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: launcherTestConfig{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installRoot: "/home/testuser/.local",
				launcherID:  "mddfmcdnhpnhoefmmiochnnjofmfhanb",
			},
		}, {
			Name:      "system_wayland",
			ExtraData: []string{"launcher_wayland_demo_fixed_size.desktop", "launcher_wayland_demo.png"},
			Val: launcherTestConfig{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installRoot: "/usr",
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			},
		}, {
			Name:      "system_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: launcherTestConfig{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installRoot: "/usr",
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
	conf := s.Param().(launcherTestConfig)
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	cont := pre.Container
	ownerID := cont.VM.Concierge.GetOwnerID()

	// Confirm we don't have the application going-in or leaving.
	if err := waitForIcon(ctx, ownerID, conf.launcherID, iconAbsent); err != nil {
		s.Fatal("Icon should not be present before installation: ", err)
	}
	defer func() {
		if err := waitForIcon(ctx, ownerID, conf.launcherID, iconAbsent); err != nil {
			s.Error("Icon should not be present after uninstallation: ", err)
		}
	}()

	iconPath := filepath.Join(conf.installRoot, "share", "icons", "hicolor", "32x32", "apps", conf.iconFile)
	if err := crostini.TransferToContainerAsRoot(ctx, cont, s.DataPath("launcher_"+conf.iconFile), iconPath); err != nil {
		s.Fatal("Failed transferring the icon: ", err)
	}
	defer crostini.RemoveContainerFile(ctx, cont, iconPath)

	desktopPath := filepath.Join(conf.installRoot, "share", "applications", conf.desktopFile)
	if err := crostini.TransferToContainerAsRoot(ctx, cont, s.DataPath("launcher_"+conf.desktopFile), desktopPath); err != nil {
		s.Fatal("Failed transferring the .desktop: ", err)
	}
	defer crostini.RemoveContainerFile(ctx, cont, desktopPath)

	// There's a delay with apps being installed in Crostini and them appearing
	// in the launcher as well as having their icons loaded. The icons are only
	// loaded after they appear in the launcher, so if we check that first we know
	// it is in the launcher afterwards.
	if err := waitForIcon(ctx, ownerID, conf.launcherID, iconExists); err != nil {
		s.Fatal("Icon should not be absent after installation: ", err)
	}

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
	if err := setAppScaled(ctx, tconn, appID, scaled); err != nil {
		return crostini.Size{}, err
	}

	if err := launchApp(ctx, tconn, appID); err != nil {
		return crostini.Size{}, err
	}

	sz, err := crostini.PollWindowSize(ctx, tconn, windowName)
	if err != nil {
		return crostini.Size{}, err
	}
	s.Log("Window size is ", sz)

	if visible, err := isAppShown(ctx, tconn, appID); err != nil {
		return crostini.Size{}, err
	} else if !visible {
		return crostini.Size{}, errors.New("App was not visible in shelf after opening")
	}

	// Close the application with a keypress.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	// This may not happen instantaneously, so poll for it.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := isAppShown(ctx, tconn, appID); err != nil {
			return err
		} else if visible {
			return errors.New("app was visible in shelf after closing")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return crostini.Size{}, err
	}
	return sz, nil
}

type iconExpectation bool

const (
	iconExists iconExpectation = true
	iconAbsent iconExpectation = false
)

// waitForIcon verifies that the Crostini icon folder for the specified
// application exists (or doesnt) in the filesystem and contains at least one file.
func waitForIcon(ctx context.Context, ownerID, appID string, expectation iconExpectation) error {
	iconDir := filepath.Join("/home/user", ownerID, "crostini.icons", appID)
	existenceCheck := func() (iconExpectation, error) {
		if fileInfo, err := os.Stat(iconDir); err != nil {
			return iconAbsent, err
		} else if !fileInfo.IsDir() {
			return iconAbsent, errors.Errorf("icon path %v is not a directory", iconDir)
		}
		entries, err := ioutil.ReadDir(iconDir)
		if err != nil {
			return iconAbsent, errors.Wrapf(err, "failed reading dir %v", iconDir)
		}
		return len(entries) > 0, errors.Errorf("icon folder has %d entries", len(entries))
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if existence, err := existenceCheck(); existence != expectation {
			return errors.Wrapf(err, "icon existence mismatched: got %v; want %v", existence, expectation)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// launchApp launches the specified application via an autotest API call.
func launchApp(ctx context.Context, tconn *chrome.Conn, appID string) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.launchApp)('%v')`, appID), nil)
}

// setAppScaled sets the specified application to be scaled or not via an autotest API call.
func setAppScaled(ctx context.Context, tconn *chrome.Conn, appID string, scaled bool) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setCrostiniAppScaled)('%v', %v)`, appID, scaled), nil)
}

// isAppShown makes an autotest API call to determine if the specified
// application has a shelf icon that is in the running state and returns true
// if so, false otherwise.
func isAppShown(ctx context.Context, tconn *chrome.Conn, appID string) (bool, error) {
	var appShown bool
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.isAppShown)('%v')`, appID)
	if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
		return false, err
	}
	return appShown, nil
}
