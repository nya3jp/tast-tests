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
		Func:     PhysicalLocation,
		Desc:     "Checks if physical location information is present for Type C connectors",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
	})
}

const typecPath = "/sys/class/typec/"

func PhysicalLocation(ctx context.Context, s *testing.State) {

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
		} else if err := checkForPhysicalLocationDir(filepath.Join(typecPath, port.Name())); err != nil {
			s.Errorf("Failed to verify proper physical location for %s: %v", port.Name(), err)
		}
	}
}

func checkForPhysicalLocationDir(portPath string) error {
	physicalLocationPath := filepath.Join(portPath, "physical_location")
	files, err := ioutil.ReadDir(physicalLocationPath)
	if err != nil {
		return errors.Errorf("could not read directory %s", physicalLocationPath)
	}

	for _, file := range files {
		filePath := filepath.Join(physicalLocationPath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("could not read file %s", filePath)
		}

		matched, err := regexp.MatchString(`^\w+\n$`, string(data))
		if err != nil {
			return err
		} else if !matched {
			return errors.Errorf("could not find a valid value in %s", file.Name())
		}

		if file.Name() == "lid" && string(data) == "yes\n" {
			return errors.Error("Type C port is not located on the lid, but the value for lid is set as yes")
		}
	}

	return nil
}
