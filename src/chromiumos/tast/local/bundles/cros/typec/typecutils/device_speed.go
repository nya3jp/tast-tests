// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typecutils

import (
	"bufio"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

type resMap map[string]interface{}

// ListDeviceSpeed returns the class, driver and speed for all the usb devices.
func ListDeviceSpeed(ctx context.Context) ([]resMap, error) {
	out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsusb command")
	}
	lsusbOut := string(out)
	reMatch := map[string]*regexp.Regexp{
		"class":  regexp.MustCompile(`Class=([a-zA-Z0-9_\-\s]+),`),
		"driver": regexp.MustCompile(`Driver=([a-zA-Z0-9_\-\/\s]+),`),
		"speed":  regexp.MustCompile(`, ([a-zA-Z0-9_\/]+)$`),
	}
	var resSlice []resMap
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		data := resMap{}
		for key, reg := range reMatch {
			match := reg.FindStringSubmatch(sc.Text())
			if match == nil {
				data[key] = ""
			} else {
				data[key] = strings.TrimSpace(match[1])
			}
		}
		resSlice = append(resSlice, data)
	}
	return resSlice, nil
}

// CheckDeviceSpeed checks mass storage device speed and return error
// if provided speed is not matched or nil if matched.
func CheckDeviceSpeed(ctx context.Context, speed string) error {
	res, err := ListDeviceSpeed(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get lsusb details")
	}
	devSpeed := ""
	flag := false
	for _, dev := range res {
		if dev["class"].(string) == "Mass Storage" {
			devSpeed = dev["speed"].(string)
			if speed == devSpeed {
				flag = true
				break
			}
		}
	}
	if !flag {
		return errors.Errorf("failed to find usb device speed;got %s, want %s", devSpeed, speed)
	}
	return nil
}
