// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sysutil provides utilities for getting system-related information.
package sysutil

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

// GetAllRemovableDevices returns the all mounted removable devices connected to DUT.
func GetAllRemovableDevices() (RemovableDevices, error) {
	const (
		mountFile              = "/proc/mounts"
		udevSerialPattern      = "==\"(.*)\""
		udevCmdForSerialNumber = "udevadm info -a -n %s | grep -iE 'ATTRS{serial}' | head -n 1"
		udevCmdForModel        = `udevadm info --name %s --query=property | grep "ID_MODEL" | head -n 1`
	)

	var mountMap []map[string]string
	cmd := exec.Command("sh", "-c", "usb-devices")
	out, _ := cmd.CombinedOutput()
	usbDevicesOutput := strings.Split(string(out), "\n\n")
	var usbDevices []map[string]string
	for _, oline := range usbDevicesOutput {
		usbDevicesMap := make(map[string]string)
		for _, line := range strings.Split(oline, "\n") {
			if strings.HasPrefix(line, "D:  Ver= ") {
				usbDevicesMap["usbType"] = strings.Split(strings.Split(line, "Ver= ")[1], " ")[0]
			} else if strings.HasPrefix(line, "S:  SerialNumber=") {
				usbDevicesMap["serial"] = strings.Split(strings.Split(line, "SerialNumber=")[1], " ")[0]
			}
		}
		if !(strings.Contains(oline, "S:  SerialNumber=")) {
			usbDevicesMap["serial"] = "nil"
		}
		usbDevices = append(usbDevices, usbDevicesMap)
	}

	fileContent, err := os.Open(mountFile)
	if err != nil {
		return RemovableDevices{}, errors.Wrap(err, "failed to open file")
	}
	defer fileContent.Close()

	scanner := bufio.NewScanner(fileContent)
	mountLine := ""
	var details []MountFileDetails

	for scanner.Scan() {
		mountLine = scanner.Text()
		internalMountMap := make(map[string]string)
		if strings.HasPrefix(strings.Split(mountLine, " ")[1], "/media/removable/") {
			internalMountMap["device"] = strings.Split(mountLine, " ")[0]
			internalMountMap["mountpoint"] = strings.Split(mountLine, " ")[1]
			internalMountMap["fsType"] = strings.Split(mountLine, " ")[2]
			internalMountMap["access"] = strings.Split(strings.Split(mountLine, " ")[3], ",")[0]

			cmd := exec.Command("sh", "-c", fmt.Sprintf(udevCmdForSerialNumber, strings.Split(mountLine, " ")[0]))
			if out, err := cmd.CombinedOutput(); err != nil {
				internalMountMap["serial"] = "nil"
			} else {
				r, _ := regexp.Compile(udevSerialPattern)
				if r.MatchString(string(out)) {
					internalMountMap["serial"] = strings.Trim(strings.Trim(strings.Split(r.FindString(string(out)), "==")[1], " \r\n\t"), "\"")
				}
			}
			cmd = exec.Command("sh", "-c", fmt.Sprintf(udevCmdForModel, strings.Split(mountLine, " ")[0]))
			if out, err := cmd.CombinedOutput(); err != nil {
				internalMountMap["model"] = "nil"
			} else {
				r, _ := regexp.Compile("ID_MODEL=(.*)")
				if r.MatchString(string(out)) {
					internalMountMap["model"] = strings.Trim(strings.Trim(strings.Split(r.FindString(string(out)), "=")[1], " \r\n\t"), "\"")
				}
			}
			mountMap = append(mountMap, internalMountMap)
		}
	}

	for _, item := range usbDevices {
		if item["serial"] != "nil" {
			for _, iItem := range mountMap {
				if iItem["serial"] != "nil" && iItem["serial"] == item["serial"] {
					if _, ok := iItem["usbType"]; !ok {
						details = append(details, MountFileDetails{Device: iItem["device"],
							Mountpoint: iItem["mountpoint"],
							FsType:     iItem["fsType"],
							Access:     iItem["access"],
							Serial:     iItem["serial"],
							UsbType:    item["usbType"],
							Model:      iItem["model"]})
					}
				}
			}
		}
	}
	return RemovableDevices{details}, nil
}

// MountFileDetails contains mounted device detailed info.
type MountFileDetails struct {
	// Device represents device partition like dev/sda, dev/sdb.
	Device string
	// Mountpoint represents removable device path(/media/removable/).
	Mountpoint string
	// FsType represents device file format type.
	FsType string
	// Access represents file access of USB device like read, write.
	Access string
	// Serial represents serial number of USB device.
	Serial string
	// UsbType represents USB version type like 2.0, 3.0, 3.10.
	UsbType string
	// Model represents USB brand model name.
	Model string
}

// RemovableDevices holds object for connected USB device details.
type RemovableDevices struct {
	// RemovableDevices holds connected USB devices details.
	RemovableDevices []MountFileDetails
}
