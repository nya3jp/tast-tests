// Copyright 2022 The ChromiumOS Authors.
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
		Func:     PhysicalLocation,
		Desc:     "Checks if physical location information is present for Type C connectors",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
		//TODO(b/233375774): Find a more scalable way to add future boards to test target
		SoftwareDeps: []string{"typec_physical_location"},
	})
}

const typecPath = "/sys/class/typec/"

// PhysicalLocation checks if valid physical location is present for typec connector devices.
func PhysicalLocation(ctx context.Context, s *testing.State) {
	ports, err := ioutil.ReadDir(typecPath)
	if err != nil {
		s.Fatal("Could not read typec directory")
	}

	for _, port := range ports {
		matched, err := regexp.MatchString(`^port\d+$`, port.Name())
		if err != nil {
			s.Errorf("Could not match regex with %s: %v", port.Name(), err)
		} else if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
		} else if err := checkForPhysicalLocationDir(filepath.Join(typecPath, port.Name())); err != nil {
			s.Errorf("Failed to verify proper physical location for %s: %v",
				port.Name(), err)
		}
	}
}

// checkForPhysicalLocationDir checks if given typec port has a physical location directory with valid fields.
func checkForPhysicalLocationDir(portPath string) error {
	physicalLocationPath := filepath.Join(portPath, "physical_location")
	files, err := ioutil.ReadDir(physicalLocationPath)
	if err != nil {
		return errors.Wrapf(err, "could not read directory %s", physicalLocationPath)
	}

	for _, file := range files {
		filePath := filepath.Join(physicalLocationPath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Wrapf(err, "could not read file %s", filePath)
		}

		d := strings.Trim(string(data), "\n")
		if matched, err := regexp.MatchString(`^\w+$`, d); err != nil {
			return errors.Wrapf(err, "could not match regex with %s in file %s", d, filePath)
		} else if !matched {
			return errors.Errorf("value %s in file %s is not valid", d, filePath)
		}

		if d == "unknown" {
			return errors.Errorf("unknown physical location value in file %s", filePath)
		}
		if file.Name() == "lid" && d == "yes" {
			return errors.New("not located on the lid, but lid set as yes")
		}
	}

	return nil
}
