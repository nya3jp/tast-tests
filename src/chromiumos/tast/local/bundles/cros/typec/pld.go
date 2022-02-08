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

const typec_path = "/sys/class/typec/"

func Pld(ctx context.Context, s *testing.State) {

	ports, err := ioutil.ReadDir(typec_path)
	if err != nil {
		s.Fatal("Could not read typec directory")
	}

	for _, port := range ports {
		matched, err := regexp.MatchString(`^port\d+$`, port.Name())
		if err != nil {
			s.Fatalf("Could not match regex with %s", port.Name())
		} else if !matched {
			s.Logf("Skipping %s since it is not a port", port.Name())
		} else if err := checkForPldSysfs(filepath.Join(typec_path, port.Name())); err != nil {
			s.Error("Failed to check PLD sysfs: ", err)
		}
	}
}

func checkForPldSysfs(port_path string) error {
	pld_path := filepath.Join(port_path, "firmware_node/pld")
	files, err := ioutil.ReadDir(pld_path)
	if err != nil {
		return errors.Errorf("could not read directory %s", pld_path)
	}

	for _, file := range files {
		file_path := filepath.Join(pld_path, file.Name())
		data, err := ioutil.ReadFile(file_path)
		if err != nil {
			return errors.Errorf("could not read file %s", file_path)
		}

		matched, err := regexp.MatchString(`^\d+\n$`, string(data))
		if err != nil {
			return err
		} else if !matched {
			return errors.Errorf("could not find a valid pld value in %s", file_path)
		}

		if file.Name() == "user_visible" && string(data) != "1\n" {
			return errors.Errorf("Port set as not visible: %s", port_path)
		}
	}

	return nil
}
