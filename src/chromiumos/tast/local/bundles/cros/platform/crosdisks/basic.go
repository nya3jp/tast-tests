// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

// verifyProp checks if the passed prop satisfies the device property
// conditions. Reports an error on failure.
func verifyProp(prop map[string]dbus.Variant) []error {
	type field struct {
		name      string
		signature string
	}
	var errs []error
	for _, f := range []field{
		{"DeviceFile", "s"},
		{"DeviceIsDrive", "b"},
		{"DeviceIsDrive", "b"},
		{"DeviceIsMediaAvailable", "b"},
		{"DeviceIsOnBootDevice", "b"},
		{"DeviceIsVirtual", "b"},
		{"DeviceIsMounted", "b"},
		{"DeviceIsReadOnly", "b"},
		{"DeviceMediaType", "u"},
		{"DeviceMountPaths", "as"},
		{"DevicePresentationHide", "b"},
		{"DeviceSize", "t"},
		{"DriveModel", "s"},
		{"IdLabel", "s"},
		{"StorageDevicePath", "s"},
		{"FileSystemType", "s"},
	} {
		if v, ok := prop[f.name]; !ok {
			errs = append(errs, errors.Errorf("%s not found in the property", f.name))
		} else if v.Signature().String() != f.signature {
			errs = append(errs, errors.Errorf("unexpected signature for %s: got %s; want %s", f.name, v.Signature().String(), f.signature))
		}
	}
	if len(errs) > 0 {
		return errs
	}

	// Hereafter type assersion should not fail thanks to godbus
	// convention.

	// DeviceFile must not be empty.
	if df := prop["DeviceFile"].Value().(string); df == "" {
		return []error{errors.New("deviceFile is empty")}
	}

	// Check if the values of DeviceIsMounted and DeviceMountPaths are
	// consistent, and any DeviceMountPaths entry is not empty.
	paths := prop["DeviceMountPaths"].Value().([]string)
	if prop["DeviceIsMounted"].Value().(bool) {
		if len(paths) == 0 {
			return []error{errors.New("prop.DeviceMountPaths should not be empty if prop.DeviceIsMounted is true")}
		}
	} else {
		if len(paths) != 0 {
			return []error{errors.New("prop.DeviceMountPaths should be empty if prop.DeviceIsMounted is false")}
		}
	}
	for _, path := range paths {
		if path == "" {
			return []error{errors.New("prop.DeviceMountPaths should not contain any empty string")}
		}
	}

	return nil
}

func testEnumerateDevices(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks) {
	s.Log("Running testEnumerateDevices")
	ds, err := cd.EnumerateDevices(ctx)
	if err != nil {
		s.Error("Failed to enumerate devices: ", err)
		return
	}

	for _, d := range ds {
		if d == "" {
			s.Error("Device returned by EnumerateDevices should be non-empty string")
			continue
		}

		prop, err := cd.GetDeviceProperties(ctx, d)
		if err != nil {
			s.Errorf("Failed to fetch device property for %s: %v", d, err)
			continue
		}

		for _, err := range verifyProp(prop) {
			s.Errorf("Failed to verify property for %s: %v", d, err)
		}
	}
}

func testNonExistentDeviceProp(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks) {
	s.Log("Running testNonExistentDeviceProp")
	const path = "/dev/nonexistent"
	if _, err := cd.GetDeviceProperties(ctx, path); err == nil {
		s.Errorf("GetDeviceProperties for %s unexpectedly succeeds", path)
	}
}

func testMountNonExistentDevice(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks) {
	s.Log("Running testMountNonExistentDevice")
	const path = "/dev/nonexistent"
	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Error("Failed to start watching MountCompleted: ", err)
		return
	}
	defer w.Close(ctx)

	if err := cd.Mount(ctx, path, "" /* filesystem type */, nil /* options */); err != nil {
		s.Error("Failed to call Mount: ", err)
		return
	}

	s.Log("Waiting for MountCompleted D-Bus signal")
	m, err := w.Wait(ctx)
	if err != nil {
		s.Error("Failed to see MountCompleted D-Bus signal: ", err)
		return
	}

	if m.SourcePath != path {
		s.Errorf("Unexpected source_path: got %q; want %q", m.SourcePath, path)
	}
	if m.MountPath != "" {
		s.Errorf("Unexpected mount_path: got %q; want %q", m.MountPath, "")
	}
}

func testBootDeviceRejected(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks) {
	s.Log("Running testBootDeviceRejected")

	ds, err := cd.EnumerateDevices(ctx)
	if err != nil {
		s.Error("Failed to enumerate devices: ", err)
		return
	}

	// Find boot device.
	dev := ""
	for _, d := range ds {
		// Note: Verification is done in testEnumerateDevices, so
		// just skip invalid property.
		prop, err := cd.GetDeviceProperties(ctx, d)
		if err != nil {
			continue
		}
		if v, ok := prop["DeviceIsOnBootDevice"].Value().(bool); ok && v {
			dev = d
			break
		}
	}
	if dev != "" {
		s.Errorf("Boot device %s was unexpectedly found", dev)
	}
}

func testUnmountNonExistentDevice(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks) {
	s.Log("Running testUnmountNonExistentDevice")

	const path = "/dev/nonexistent"
	err := cd.Unmount(ctx, path, nil /* options */)
	if !errors.Is(err, crosdisks.MountErrorPathNotMounted) {
		s.Errorf("Unexpected Unmount status: got %v; want %v", err, crosdisks.MountErrorPathNotMounted)
	}
}

// RunBasicTests runs a series of tests.
func RunBasicTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}

	testEnumerateDevices(ctx, s, cd)
	testNonExistentDeviceProp(ctx, s, cd)
	testMountNonExistentDevice(ctx, s, cd)
	testBootDeviceRejected(ctx, s, cd)
	testUnmountNonExistentDevice(ctx, s, cd)
}
