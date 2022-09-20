// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"context"
	"reflect"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func testVolumeRename(ctx context.Context, cd *crosdisks.CrosDisks, ld *crosdisks.LoopbackDevice) error {
	// Mount and then unmount through the CrosDisks API to ensure correct permissions on the device, which depend on the filesystem type.
	if err := WithMountDo(ctx, cd, ld.DevicePath(), "", nil, func(ctx context.Context, mountPath string, readOnly bool) error {
		return nil
	}); err != nil {
		return err
	}

	const newName = "NEWNAME"

	// Verify the originally name was different.
	props, err := cd.GetDeviceProperties(ctx, ld.DevicePath())
	if err != nil {
		return errors.Wrapf(err, "failed to query initial properties of %q", ld.DevicePath())
	}
	label, ok := props["IdLabel"]
	if !ok {
		return errors.Errorf("failed to obtain initial IdLabel property on %q", ld.DevicePath())
	}
	if label.Signature() != dbus.SignatureOfType(reflect.TypeOf("")) || label.Value() == newName {
		return errors.Errorf("incorrect filesystem label: got %q; want NOT %q", label.String(), newName)
	}

	st, err := cd.RenameAndWaitForCompletion(ctx, ld.DevicePath(), newName)
	if err != nil {
		return errors.Wrapf(err, "failed to invoke Rename on %q", ld.DevicePath())
	}

	if st.Device != ld.DevicePath() {
		return errors.Errorf("unexpected device in response: got %q; want %q", st.Device, ld.DevicePath())
	}

	if st.Status != 0 {
		return errors.Errorf("unexpected status of rename: got %d; want 0", st.Status)
	}

	// Double-check that the name actually changed.
	props, err = cd.GetDeviceProperties(ctx, ld.DevicePath())
	if err != nil {
		return errors.Wrapf(err, "failed to query properties of %q", ld.DevicePath())
	}
	label, ok = props["IdLabel"]
	if !ok {
		return errors.Errorf("failed to obtain IdLabel property on %q", ld.DevicePath())
	}
	if label.Signature() != dbus.SignatureOfType(reflect.TypeOf("")) || label.Value() != newName {
		return errors.Errorf("incorrect filesystem label: got %q; want %q", label.String(), newName)
	}
	return nil
}

// RunRenameTests executes a set of tests which rename different filesystem partitions using CrosDisks.
func RunRenameTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	err = WithLoopbackDeviceDo(ctx, cd, loopbackSizeBytes, "", func(ctx context.Context, ld *crosdisks.LoopbackDevice) error {
		// Check that we can rename volumes.
		s.Run(ctx, "vfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.vfat -n EMPTY1", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testVolumeRename(ctx, cd, ld); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		s.Run(ctx, "exfat", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.exfat -n EMPTY2", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testVolumeRename(ctx, cd, ld); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		s.Run(ctx, "ntfs", func(ctx context.Context, state *testing.State) {
			if err := formatDevice(ctx, "mkfs.ntfs -f -L EMPTY3", ld.DevicePath()); err != nil {
				state.Fatal("Could not format device: ", err)
			}
			if err := testVolumeRename(ctx, cd, ld); err != nil {
				state.Fatal("Test case failed: ", err)
			}
		})
		return nil
	})
	if err != nil {
		s.Fatal("Failed to initialize loopback device: ", err)
	}
}
