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
			if !isSymlinkToUsbPort(d, filepath.Join(typecPortPath, usbPort.Name())) {
				return errors.Errorf("usb port directory, %s not linked to usb port device", usbPort.Name())
			}
			foundUsbPortDir = true
			if err := checkForConnectorDir(d, filepath.Join(typecPath, usbPort.Name())); err != nil {
				return err
			}

		}

		matched, err := regexp.MatchString(`^usb\d+_port\d+`, usbPort.Name())
		if err == nil && matched {
			if !isSymlinkToUsb4Port(d, filepath.Join(typecPortPath, usbPort.Name())) {
				return errors.Errorf("usb4 port directory, %s not linked to a usb4 port device", usbPort.Name())

			}
			if err := checkForConnectorDir(d, filepath.Join(typecPath, usbPort.Name())); err != nil {
				return err
			}
		}
	}

	if !foundUsbSymlink {
		return errors.Errorf("could not find any usb port directory in %s", typecPortPath)
	}
	return nil
}

// isSymlinkToUsbPort checks if absolute path of given usbPortPath is a path to a usb port device.
func isSymlinkToUsbPort(d *dut.DUT, usbPortPath string) bool {
	absPath, err := d.Conn().CommandContext(ctx, "readlink", "-f", usbPortPath).Output()
	matched, err := regexp.MatchString(`^.*\/usb\d+\/\d+-\d+:1\.0\/.*`, absPath)
	return err == nil && matched
}

// isSymlinkToUsb4Port checks if absolute path of given usb4PortPath is a path to a usb4 port device.
func isSymlinkToUsb4Port(d *dut.DUT, usb4PortPath string) bool {
	absPath, err := d.Conn().CommandContext(ctx, "readlink", "-f", usb4PortPath).Output()
	matched, err := regexp.MatchString(`^.*\/domain\d+\/\d+-\d+\/.*`, absPath)
	return err == nil && matched
}

// checkForConnectorDir checks if the given typecPortPath contains a connector directory linked to a corresponding typec connector.
func checkForConnectorDir(d *dut.DUT, usbPortPath string) error {
	ls, err := ioutil.ReadDir(usbPortPath)
	if err != nil {
		return errors.Error("could not read directory %s", usbPortPath)
	}

	for _, connector := range ls {
		if connector.Name() == "connector" {
			if isSymlinkToTypecConn(d, filepath.Join(usbPortPath, connector.Name())) {
				return errors.Errorf("connector directory in %s not linked to a typec connector device", usbPortPath)
			}
			return nil
		}
	}

	return errors.Errorf("could not find connector directory in %s", usbPortPath)
}

// isSymlinkToTypecConn checks if absolute path of given connectorPath is a path to a typec connector device.
func isSymlinkToTypecConn(d *dut.DUT, connectorPath string) bool {
	absPath, err := d.Conn().CommandContext(ctx, "readlink", "-f", connectorPath).Output()
	matched, err := regexp.MatchString(`^.*\/typec/.*`, absPath)
	return err == nil && matched
}
