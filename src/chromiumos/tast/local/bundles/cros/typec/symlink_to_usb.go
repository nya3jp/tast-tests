// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SymlinkToUSB,
		Desc:         "Checks if Type C connectors have symlink to corresponding USB ports and vice versa",
		Contacts:     []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:         []string{"group:mainline", "group:typec", "informational"},
		SoftwareDeps: []string{"typec_usb_link"},
	})
}

const typecPath = "/sys/class/typec/"

// SymlinkToUSB checks the symlink from typec connector to usb port and from usb port to typec connector
// typec conn -> usb port -> typec conn
func SymlinkToUSB(ctx context.Context, s *testing.State) {
	ls, err := ioutil.ReadDir(typecPath)
	if err != nil {
		s.Fatal("Could not read typec directory: ", err)
	}

	for _, port := range ls {
		if matched, err := regexp.MatchString(`^port\d+$`, port.Name()); err != nil {
			s.Errorf("Could not match regex with %s: %v", port.Name(), err)
		} else if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
		} else if err := checkTypecPortDir(filepath.Join(typecPath, port.Name())); err != nil {
			s.Errorf("Failed to verify proper symlink within %s: %v", port.Name(), err)
		}
	}
}

// checkTypecPortDir checks if the given typecPortPath contains a proper usb port directory.
func checkTypecPortDir(typecPortPath string) error {
	ls, err := ioutil.ReadDir(typecPortPath)
	if err != nil {
		return errors.Wrapf(err, "could not read directory %s", typecPortPath)
	}

	// foundUsbPortDir keeps track of only USB port (required) but not USB4 port (optional)
	foundUsbPortDir := false
	for _, usbPort := range ls {
		usbPortPath := filepath.Join(typecPortPath, usbPort.Name())
		if matched, err := regexp.MatchString(`^.*\/usb\d+-port\d+`, usbPortPath); err == nil && matched {
			if err := checkUsbPortDir(usbPortPath, false); err != nil {
				return err
			}
			foundUsbPortDir = true
		}

		// USB4 port
		if matched, err := regexp.MatchString(`^.*\/usb\d+_port\d+`, usbPortPath); err == nil && matched {
			if err := checkUsbPortDir(usbPortPath, true); err != nil {
				return err
			}
		}
	}

	if !foundUsbPortDir {
		return errors.Errorf("could not find any usb port directory in %s", typecPortPath)
	}
	return nil
}

// checkUsbPortDir checks if the given usbPortPath is linked to a usb port device and if it contains a proper connector directory.
func checkUsbPortDir(usbPortPath string, isUsb4 bool) error {
	absolutePath, err := filepath.EvalSymlinks(usbPortPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink for %s", usbPortPath)
	}

	absolutePathRegex := `^.*\/usb\d+\/\d+-\d+:1\.0\/.*`
	if isUsb4 {
		absolutePathRegex = `^.*\/domain\d+\/\d+-\d+\/.*`
	}

	matched, err := regexp.MatchString(absolutePathRegex, string(absolutePath))
	if err != nil {
		return errors.Wrapf(err, "could not match regex with %s", string(absolutePath))
	}
	if !matched {
		return errors.Errorf("usb port directory, %s, not linked to usb port device", usbPortPath)
	}

	ls, err := ioutil.ReadDir(absolutePath)
	if err != nil {
		return errors.Wrapf(err, "could not read directory %s", usbPortPath)
	}

	for _, connector := range ls {
		if connector.Name() == "connector" {
			return checkConnectorDir(filepath.Join(usbPortPath, connector.Name()))
		}
	}

	return errors.Errorf("could not find connector directory in %s", usbPortPath)
}

// checkConnectorDir checks if the given connectorPath is linked to a typec connector device.
func checkConnectorDir(connectorPath string) error {
	absolutePath, err := filepath.EvalSymlinks(connectorPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink for %s", connectorPath)
	}

	matched, err := regexp.MatchString(`^.*\/typec/.*`, string(absolutePath))
	if err != nil {
		return errors.Wrapf(err, "could not match regex with %s", string(absolutePath))
	}
	if !matched {
		return errors.Errorf("connector directory, %s, not linked to typec connector device", connectorPath)
	}

	return nil
}
