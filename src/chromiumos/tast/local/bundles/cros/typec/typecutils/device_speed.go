// Copyright 2022 The ChromiumOS Authors.
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

// re will parse Class, Driver and Speed of USB devices from 'lsusb -t' command output.
// Sample output of 'lsusb -t' command is as below:
/*
/:  Bus 04.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/4p, 10000M
/:  Bus 03.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/12p, 480M
    |__ Port 2: Dev 2, If 0, Class=Mass Storage, Driver=usb-storage, 5000M
    |__ Port 2: Dev 2, If 0, Class=Vendor Specific Class, Driver=asix, 480M
    |__ Port 5: Dev 3, If 0, Class=Video, Driver=uvcvideo, 480M
    |__ Port 5: Dev 3, If 1, Class=Video, Driver=uvcvideo, 480M
    |__ Port 10: Dev 4, If 0, Class=Wireless, Driver=btusb, 12M
    |__ Port 10: Dev 4, If 1, Class=Wireless, Driver=btusb, 12M
/:  Bus 02.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/4p, 10000M
/:  Bus 01.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/1p, 480M
*/
var re = regexp.MustCompile(`.*Class=([a-zA-Z_\s]+).*Driver=([a-zA-Z0-9_\-\/\s]+).*,.([a-zA-Z0-9_\/.]+)`)

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
		return errors.Wrap(err, "failed to get file info")
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
