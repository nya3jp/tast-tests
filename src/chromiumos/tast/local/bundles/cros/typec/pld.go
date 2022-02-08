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
		Func:     Pld,
		Desc:     "Checks if PLD information is present for Type C connectors",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
	})
}

const typecPath = "/sys/class/typec/"

func Pld(ctx context.Context, s *testing.State) {

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
		} else if err := checkForPldSysfs(filepath.Join(typecPath, port.Name())); err != nil {
			s.Errorf("Failed to verify proper PLD sysfs for %s: %v", port.Name(), err)
		}
	}
}

func checkForPldSysfs(portPath string) error {
	pldPath := filepath.Join(portPath, "firmware_node/pld")
	files, err := ioutil.ReadDir(pldPath)
	if err != nil {
		return errors.Errorf("could not read directory %s", pldPath)
	}

	for _, file := range files {
		filePath := filepath.Join(pldPath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("could not read file %s", filePath)
		}

		matched, err := regexp.MatchString(`^\d+\n$`, string(data))
		if err != nil {
			return err
		} else if !matched {
			return errors.Errorf("could not find a valid pld value in %s", file.Name())
		}

		if file.Name() == "user_visible" && string(data) == "0\n" {
			return errors.Error("port set as not visible")
		}
	}

	return nil
}
