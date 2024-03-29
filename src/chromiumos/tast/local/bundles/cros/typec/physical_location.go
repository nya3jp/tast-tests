// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalLocation,
		Desc:         "Checks if physical location information is present for Type C connectors",
		Contacts:     []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:         []string{"group:mainline", "group:typec", "informational"},
		SoftwareDeps: []string{"typec_physical_location"},
	})
}

// PhysicalLocation checks if valid physical location is present for typec connector devices.
func PhysicalLocation(ctx context.Context, s *testing.State) {
	const typecPath = "/sys/class/typec/"

	ports, err := ioutil.ReadDir(typecPath)
	if err != nil {
		s.Fatal("Could not read typec directory")
	}

	for _, port := range ports {
		matched, err := regexp.MatchString(`^port\d+$`, port.Name())
		if err != nil {
			s.Fatalf("Could not match regex with %s: %v", port.Name(), err)
		}
		if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
			continue
		}
		if err := checkForPhysicalLocationDir(filepath.Join(typecPath, port.Name())); err != nil {
			s.Fatalf("Failed to verify proper physical location for %s: %v",
				port.Name(), err)
		}
	}
}

// checkForPhysicalLocationDir checks if given typec port has a physical location directory with valid fields.
func checkForPhysicalLocationDir(portPath string) error {
	physicalLocationPath := filepath.Join(portPath, "physical_location")

	if panel, err := readFileFromDir(physicalLocationPath, "panel"); err != nil {
		return err
	} else if panel != "top" &&
		panel != "bottom" &&
		panel != "left" &&
		panel != "right" &&
		panel != "front" &&
		panel != "back" {
		return errors.Errorf("invalid panel value: %s", panel)
	}

	if horizontalPosition, err := readFileFromDir(physicalLocationPath, "horizontal_position"); err != nil {
		return err
	} else if horizontalPosition != "left" &&
		horizontalPosition != "center" &&
		horizontalPosition != "right" {
		return errors.Errorf("invalid horizontal_position value: %s", horizontalPosition)
	}

	if verticalPosition, err := readFileFromDir(physicalLocationPath, "vertical_position"); err != nil {
		return err
	} else if verticalPosition != "upper" &&
		verticalPosition != "center" &&
		verticalPosition != "bottom" {
		return errors.Errorf("invalid vertical_position value: %s", verticalPosition)
	}

	return nil
}

// readFileFromDir returns the content of the file within the directory.
func readFileFromDir(directory, file string) (string, error) {
	path := filepath.Join(directory, file)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "could not read file %s", path)
	}
	d := strings.Trim(string(data), "\n")
	return d, nil
}
