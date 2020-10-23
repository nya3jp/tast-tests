// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var devRegExp = regexp.MustCompile(`(sda|nvme\dn\d|mmcblk\d)$`)

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
	Blockdevices []*Blockdevice `json:"blockdevices"`
}

// MainDevice returns the main storage device from a list of available devices.
// The method returns the device with the biggest size if multiple present.
func (d DiskInfo) MainDevice() (*Blockdevice, error) {
	var bestMatch *Blockdevice
	for _, device := range d.Blockdevices {
		if bestMatch == nil || bestMatch.Size < device.Size {
			bestMatch = device
		}
	}
	if bestMatch == nil {
		return nil, errors.Errorf("unable to identify main storage device from devices: %+v", d)
	}
	return bestMatch, nil
}

// SlcDevice returns the slc storage device from a list of available
// devices. The method assumes at most two devices and returns the device
// with the smallest size.
func (d DiskInfo) SlcDevice() (*Blockdevice, error) {
	if d.DeviceCount() < 2 {
		return nil, errors.Errorf("no secondary devices present: %+v", d)
	}
	var bestMatch *Blockdevice
	for _, device := range d.Blockdevices {
		if bestMatch == nil || bestMatch.Size > device.Size {
			bestMatch = device
		}
	}
	return bestMatch, nil
}

// DeviceCount returns number of found valid block devices on the system.
func (d DiskInfo) DeviceCount() int {
	return len(d.Blockdevices)
}

// CheckMainDeviceSize verifies that the size of the main storage disk is more than
// the given minimal size. Otherwise, an error is returned.
func (d DiskInfo) CheckMainDeviceSize(minSize int64) error {
	device, err := d.MainDevice()
	if err != nil {
		return errors.Wrap(err, "failed getting main storage disk")
	}

	if device.Size < minSize {
		return errors.Errorf("main storage device size too small: %v", device.Size)
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

// SizeInGB returns size of the main block device in whole GB's.
func (d DiskInfo) SizeInGB() (int, error) {
	device, err := d.MainDevice()
	if err != nil {
		return 0, errors.Wrap(err, "failed getting main storage disk")
	}

	return int(math.Round(float64(device.Size) / 1e9)), nil
}

// PartitionSize return size (in bytes) of given disk partition.
func PartitionSize(ctx context.Context, partition string) (uint64, error) {
	devNames := strings.Split(partition, "/")
	cmd := fmt.Sprintf("cat /proc/partitions | egrep '%v$' | awk '{print $3}'", devNames[len(devNames)-1])
	out, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return 0, errors.Wrapf(err, "failed checking size of partition: %s", partition)
	}
	blocks, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed parsing size of partition: %s", partition)
	}
	return uint64(blocks) * 1024, nil
}

// RootPartitionForTrim returns root partition for trim stress.
func RootPartitionForTrim(ctx context.Context) (string, error) {
	diskName, err := fixedDstDrive(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed selecting free root partition")
	}

	rootDev, err := rootDevice(ctx, false)
	if err != nil {
		return "", errors.Wrap(err, "failed selecting free root partition")
	}

	testing.ContextLog(ctx, "Diskname: ", diskName, ", root: ", rootDev)
	if diskName == rootDev {
		freeRootPart, err := freeRootPartition(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed selecting free root partition")
		}
		return freeRootPart, nil
	}

	return diskName, nil
}

func fixedDstDrive(ctx context.Context) (string, error) {
	command := ". /usr/sbin/write_gpt.sh;. /usr/share/misc/chromeos-common.sh;load_base_vars;get_fixed_dst_drive"
	out, err := testexec.CommandContext(ctx, "sh", "-c", command).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to read fixed DST drive info")
	}
	return strings.TrimSpace(string(out)), nil
}

func rootDevice(ctx context.Context, returnPartitionName bool) (string, error) {
	args := []string{"-s"}
	if !returnPartitionName {
		args = append(args, "-d")
	}
	out, err := testexec.CommandContext(ctx, "rootdev", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to read root device info")
	}
	return strings.TrimSpace(string(out)), nil
}

func freeRootPartition(ctx context.Context) (string, error) {
	partition, err := rootDevice(ctx, true)
	if err != nil {
		return "", errors.Wrap(err, "failed to read root partition info")
	}
	testing.ContextLog(ctx, "Partition: ", partition)
	spareRootMap := map[string]string{"3": "5", "5": "3"}
	return partition[:len(partition)-1] + spareRootMap[partition[len(partition)-1:]], nil
}

// ReadDiskInfo returns storage information as reported by lsblk tool.
// Only disk devices are returns.
func ReadDiskInfo(ctx context.Context) (*DiskInfo, error) {
	cmd := testexec.CommandContext(ctx, "lsblk", "-b", "-d", "-J", "-o", "NAME,TYPE,HOTPLUG,SIZE,STATE")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsblk")
	}
	diskInfo, err := parseDiskInfo(out)
	if err != nil {
		return nil, err
	}
	return removeDisallowedDevices(diskInfo), nil
}

func parseDiskInfo(out []byte) (*DiskInfo, error) {
	var result DiskInfo
	// TODO(dlunev): make sure the format is the same for all kernel versions.
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse lsblk result")
	}
	return &result, nil
}

// removeDisallowedDevices filters out devices which are not matching the regexp
// or are not disks
// TODO(dlunev): We should consider mmc devices only if they are 'root' devices
// for there is no reliable way to differentiate removable mmc.
func removeDisallowedDevices(diskInfo *DiskInfo) *DiskInfo {
	var devices []*Blockdevice
	for _, device := range diskInfo.Blockdevices {
		if device.Type == "disk" && devRegExp.MatchString(device.Name) {
			devices = append(devices, device)
		}
	}
	return &DiskInfo{Blockdevices: devices}
}
