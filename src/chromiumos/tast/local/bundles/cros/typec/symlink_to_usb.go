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
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SymlinkToUSB,
		Desc:     "Checks if Type C connectors have symlink to corresponding USB ports and vice versa",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
	})
}

const typecPath = "/sys/class/typec/"

func SymlinkToUSB(ctx context.Context, s *testing.State) {
	ver, _, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version: ", err)
	}

	if !ver.IsOrLater(5, 10) {
		s.Log("Kernel version is too old for symlink support between typec conn and usb port: ", ver)
		return
	}

	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	ports, err := ioutil.ReadDir(typecPath)
	if err != nil {
		s.Fatal("Could not read typec directory")
	}

	for _, port := range ports {
		matched, err := regexp.MatchString(`^port\d+$`, port.Name())
		if err != nil {
			s.Fatalf("Could not match regex with %s", port.Name())
		} else if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
		} else if err := checkForUsbPortDir(d, filepath.Join(typecPath, port.Name())); err != nil {
			s.Errorf("Failed to verify proper symlink within %s: %v", port.Name(), err)
		}
	}
}

// checkForUsbPortDir checks if the given typecPortPath contains a usb port directory linked to a corresponding usb port device.
func checkForUsbPortDir(d *dut.DUT, typecPortPath string) error {
	ls, err := ioutil.ReadDir(typecPortPath)
	if err != nil {
		return errors.Error("could not read directory %s", typecPortPath)
	}

	foundUsbPortDir := false
	for _, usbPort := range ls {
		matched, err := regexp.MatchString(`^usb\d+-port\d+`, usbPort.Name())
		if err == nil && matched {
			if !isSymlink(d, filepath.Join(typecPortPath, usbPort.Name()), `^.*\/usb\d+\/\d+-\d+:1\.0\/.*`) {
				return errors.Errorf("usb port directory, %s not linked to usb port device", usbPort.Name())
			}
			foundUsbPortDir = true
			if err := checkForConnectorDir(d, filepath.Join(typecPath, usbPort.Name())); err != nil {
				return err
			}

		}

		matched, err := regexp.MatchString(`^usb\d+_port\d+`, usbPort.Name())
		if err == nil && matched {
			if !isSymlink(d, filepath.Join(typecPortPath, usbPort.Name()), `^.*\/domain\d+\/\d+-\d+\/.*`) {
				return errors.Errorf("usb4 port directory, %s not linked to a usb4 port device", usbPort.Name())

			}
			if err := checkForConnectorDir(d, filepath.Join(typecPath, usbPort.Name())); err != nil {
				return err
			}
		}
	}

	if !foundUsbPortDir {
		return errors.Errorf("could not find any usb port directory in %s", typecPortPath)
	}
	return nil
}

// checkForConnectorDir checks if the given typecPortPath contains a connector directory linked to a corresponding typec connector.
func checkForConnectorDir(d *dut.DUT, usbPortPath string) error {
	ls, err := ioutil.ReadDir(usbPortPath)
	if err != nil {
		return errors.Error("could not read directory %s", usbPortPath)
	}

	for _, connector := range ls {
		if connector.Name() == "connector" {
			if isSymlink(d, filepath.Join(usbPortPath, connector.Name()), `^.*\/typec/.*`) {
				return errors.Errorf("connector directory in %s not linked to a typec connector device", usbPortPath)
			}
			return nil
		}
	}

	return errors.Errorf("could not find connector directory in %s", usbPortPath)
}

// isSymlink returns true if given path is a symlink matching absolutePathRegex.
func isSymlink(d *dut.DUT, path, absolutePathRegex string) bool {
	absolutePath, err := d.Conn().CommandContext(ctx, "readlink", "-f", path).Output()
	matched, err := regexp.MatchString(absolutePathRegex, absolutePath)
	return err == nil && matched
}
