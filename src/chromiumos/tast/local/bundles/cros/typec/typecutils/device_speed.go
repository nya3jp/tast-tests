// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typecutils

import (
	"bufio"
	"context"
	"io"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// USB represents information of all USB devices.
type USB struct {
	// Class represents class that the connected device falls into. (Example: Mass storage, Wireless, etc).
	Class string
	// Driver represents driver that drives the connected device. (Example: hub, btusb, etc).
	Driver string
	// Speed represents the speed of connected device. (Example: 480M, 5000M, etc).
	Speed string
}

var re = regexp.MustCompile(`.*Class=([a-zA-Z_\s]+).*Driver=([a-zA-Z0-9_\-\/\s]+).*,.([a-zA-Z0-9_\/]+)`)

// ListDevicesInfo returns the class, driver and speed for all the USB devices.
func ListDevicesInfo(ctx context.Context) ([]USB, error) {
	out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsusb command")
	}
	lsusbOut := string(out)
	var res []USB
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		match := re.FindStringSubmatch(sc.Text())
		if match == nil {
			continue
		}
		res = append(res, USB{
			Class:  match[1],
			Driver: match[2],
			Speed:  match[3],
		})
	}
	return res, nil
}

// MassStorageUSBSpeed returns mass storage device speed for all USB devices.
// If failed to get devices speed returns error.
func MassStorageUSBSpeed(ctx context.Context) ([]string, error) {
	res, err := ListDevicesInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get lsusb details")
	}
	var speedSlice []string
	for _, dev := range res {
		if dev.Class == "Mass Storage" {
			devSpeed := dev.Speed
			if devSpeed != "" {
				speedSlice = append(speedSlice, devSpeed)
			}
		}
	}
	if len(speedSlice) == 0 {
		return nil, errors.New("failed to find USB device speed")
	}
	return speedSlice, nil
}

// CopyFile performs copying of file from given source to destination.
func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return errors.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return errors.Wrap(err, "failed to copy")
	}
	return nil
}
