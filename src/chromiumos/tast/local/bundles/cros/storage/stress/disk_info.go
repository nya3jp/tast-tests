// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Blockdevice represents information about a single storage device as reported by lsblk.
type Blockdevice struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Hotplug string `json:"hotplug"`
	Size    int64  `json:"size,string"`
	State   string `json:"state"`
}

// DiskInfo is a serializable structure representing output of lsblk command.
type DiskInfo struct {
	Blockdevices []Blockdevice `json:"blockdevices"`
}

// GetMainDevice returns the main storage device from a list of available devices.
// This method assumes there is only one root disk device that matches naming pattern.
// Otherwise, the first matching device is returned.
func (d DiskInfo) GetMainDevice() (*Blockdevice, error) {
	mainDevRegExp := regexp.MustCompile(`sda|nvme|mmcblk`)
	for _, device := range d.Blockdevices {
		if mainDevRegExp.MatchString(device.Name) {
			return &device, nil
		}
	}
	return nil, errors.Errorf("Unable to identify main storage device from devices: %+v", d)
}

// CheckMainDeviceSize verifies that the size of the main storage disk is more than
// the given minimal size. Otherwise, an error is returned.
func (d DiskInfo) CheckMainDeviceSize(minSize int64) error {
	device, err := d.GetMainDevice()
	if err != nil {
		return errors.Wrap(err, "failed getting main storage disk")
	}

	if device.Size < minSize {
		return errors.Errorf("Main storage device size too small: %v", device.Size)
	}
	return nil
}

// SaveDiskInfo dumps disk info to an external file with a given file name.
// The information is saved in JSON format.
func (d DiskInfo) SaveDiskInfo(fileName string) error {
	file, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling disk info to JSON")
	}
	err = ioutil.WriteFile(fileName, file, 0644)
	if err != nil {
		return errors.Wrap(err, "failed saving disk info to file")
	}
	return nil
}

// GetDiskInfo returns storage information as reported by lsblk tool.
// Only disk devices are returns.
func GetDiskInfo(ctx context.Context) (*DiskInfo, error) {
	cmd := testexec.CommandContext(ctx, "lsblk", "-b", "-d", "-J", "-o", "NAME,TYPE,HOTPLUG,SIZE,STATE")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsblk")
	}
	diskInfo, err := parseDiskInfo(out)
	if err != nil {
		return nil, err
	}
	return filterNonDiskDevices(diskInfo), nil
}

func parseDiskInfo(out []byte) (*DiskInfo, error) {
	var result DiskInfo
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse lsblk result")
	}
	return &result, nil
}

func filterNonDiskDevices(diskInfo *DiskInfo) *DiskInfo {
	var devices []Blockdevice
	for _, device := range diskInfo.Blockdevices {
		if device.Type == "disk" {
			devices = append(devices, device)
		}
	}
	return &DiskInfo{Blockdevices: devices}
}
