// Copyright 2022 The ChromiumOS Authors.
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
		Func:         SymlinkToUsb,
		Desc:         "Checks if Type C connectors have symlink to corresponding USB ports and vice versa",
		Contacts:     []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:         []string{"group:mainline", "group:typec", "informational"},
		SoftwareDeps: []string{"typec_usb_link"},
	})
}

const typecPath = "/sys/class/typec/"

// SymlinkToUsb checks the symlink from typec connector to usb port and from usb port to typec connector
// typec conn -> usb port -> typec conn
func SymlinkToUsb(ctx context.Context, s *testing.State) {
	ls, err := ioutil.ReadDir(typecPath)
	if err != nil {
		s.Fatal("Could not read typec directory: ", err)
	}

	for _, port := range ls {
		if matched, err := regexp.MatchString(`^port\d+$`, port.Name()); err != nil {
			s.Fatalf("Could not match regex with %s: %v", port.Name(), err)
		} else if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
		} else if err := checkTypecPortDir(filepath.Join(typecPath, port.Name())); err != nil {
			s.Fatalf("Failed to verify proper symlink within %s: %v", port.Name(), err)
		}
	}
}

// checkTypecPortDir checks if the given typecPortPath contains a proper usb port directory.
func checkTypecPortDir(typecPortPath string) error {
	typecPortAbsPath, err := filepath.EvalSymlinks(typecPortPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink for %s", typecPortPath)
	}

	ls, err := ioutil.ReadDir(typecPortPath)
	if err != nil {
		return errors.Wrapf(err, "could not read directory %s", typecPortPath)
	}

	var usbPorts []string
	for _, file := range ls {
		if matched, err := regexp.MatchString(`^usb\d+(-|_)port\d+`, file.Name()); err != nil {
			return errors.Wrapf(err, "could not match regex with %s", file.Name())
		} else if matched {
			usbPorts = append(usbPorts, file.Name())
		}
	}
	if len(usbPorts) == 0 {
		return errors.Errorf("could not find any usb port directory in %s", typecPortPath)
	}

	for _, usbPort := range usbPorts {
		usbPortPath := filepath.Join(typecPortPath, usbPort)
		if err := checkUsbPortDir(usbPortPath, typecPortAbsPath); err != nil {
			return err
		}
	}

	return nil
}

// checkUsbPortDir checks if the given usbPortPath is linked to a usb port device and if it contains a proper connector directory linked back to typec port.
func checkUsbPortDir(usbPortPath, typecPortAbsPath string) error {
	usbPortAbsPath, err := filepath.EvalSymlinks(usbPortPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink for %s", usbPortPath)
	}

	// USB port absolute path example: /sys/devices/pci0000:00/0000:00:0d.0/usbX/X-X:X.X/usbX-portX
	// USB4 port absolute path example: /sys/devices/pci0000:00/0000:00:0d.2/domainX/X-X/usbX_portX
	usbPortAbsPathRegex := `^.*\/(usb|domain)\d+\/\d+-\d+.*`
	matched, err := regexp.MatchString(usbPortAbsPathRegex, usbPortAbsPath)
	if err != nil {
		return errors.Wrapf(err, "could not match regex with %s", usbPortAbsPath)
	}
	if !matched {
		return errors.Errorf("usb port directory, %s, not linked to usb port device", usbPortPath)
	}

	ls, err := ioutil.ReadDir(usbPortAbsPath)
	if err != nil {
		return errors.Wrapf(err, "could not read directory %s", usbPortPath)
	}

	for _, file := range ls {
		if file.Name() != "connector" {
			continue
		}

		connectorPath := filepath.Join(usbPortPath, file.Name())
		if connectorAbsPath, err := filepath.EvalSymlinks(connectorPath); err != nil {
			return errors.Wrapf(err, "could not evaluate symlink for %s", connectorPath)
		} else if connectorAbsPath != typecPortAbsPath {
			return errors.Errorf("connector directory, %s, not linked to typec connector device", connectorPath)
		}
		return nil
	}

	return errors.Errorf("could not find connector directory in %s", usbPortPath)
}
