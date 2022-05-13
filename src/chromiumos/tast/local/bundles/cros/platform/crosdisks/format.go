// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"context"
	"reflect"
	"strings"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func testVolumeFormat(ctx context.Context, cd *crosdisks.CrosDisks, ld *crosdisks.LoopbackDevice, fsType string) error {
	label := "LB" + strings.ToUpper(fsType)
	st, err := cd.FormatAndWaitForCompletion(ctx, ld.DevicePath(), fsType, []string{"Label", label})
	if err != nil {
		return errors.Wrapf(err, "failed to invoke Format on %q as %q", ld.DevicePath(), fsType)
	}

	if st.Device != ld.DevicePath() {
		return errors.Errorf("unexpected device in response: got %q; want %q", st.Device, ld.DevicePath())
	}

	if st.Status != 0 {
		return errors.Errorf("unexpected status of format: got %d; want 0", st.Status)
	}

	// Double-check if the device has correct label now.
	props, err := cd.GetDeviceProperties(ctx, ld.DevicePath())
	if err != nil {
		return errors.Wrapf(err, "failed to query properties of %q", ld.DevicePath())
	}
	actualLabel, ok := props["IdLabel"]
	if !ok {
		return errors.Errorf("failed to obtain IdLabel property on %q", ld.DevicePath())
	}
	if actualLabel.Signature() != dbus.SignatureOfType(reflect.TypeOf("")) {
		return errors.Errorf("incorrect filesystem label format %q", actualLabel.String())
	}
	if actualLabel.Value() != label {
		return errors.Errorf("incorrect filesystem label: got %q; want %q", actualLabel.Value(), label)
	}
	return nil
}

// RunFormatTests executes a set of tests which format a partition with different filesystems CrosDisks.
func RunFormatTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	for _, fsType := range []string{"vfat", "exfat", "ntfs"} {
		s.Run(ctx, fsType, func(ctx context.Context, s *testing.State) {
			err = withLoopbackDeviceDo(ctx, cd, loopbackSizeBytes, "", func(ctx context.Context, ld *crosdisks.LoopbackDevice) error {
				if err := testVolumeFormat(ctx, cd, ld, fsType); err != nil {
					s.Error("Test case failed: ", err)
				}
				return nil
			})
			if err != nil {
				s.Error("Failed to initialize loopback device: ", err)
			}
		})
	}
}
