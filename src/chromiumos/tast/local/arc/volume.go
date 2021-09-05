// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// RemovableMediaUUID is the (fake) UUID of the removable media volume for
	// testing. It is defined in
	// Chromium's components/arc/volume_mounter/arc_volume_mounter_bridge.cc.
	RemovableMediaUUID = "00000000000000000000000000000000DEADBEEF"

	// MyFilesUUID is the UUID of the ARC MyFiles volume. It is defined in
	// Chromium's components/arc/volume_mounter/arc_volume_mounter_bridge.cc.
	MyFilesUUID = "0000000000000000000000000000CAFEF00D2019"

	// VolumeProviderContentURIPrefix is the prefix of the URIs of files served by
	// ArcVolumeProvider.
	VolumeProviderContentURIPrefix = "content://org.chromium.arc.volumeprovider/"
)

// waitForARCVolumeMount waits for a volume to be mounted in ARC using the sm
// command. Just checking mountinfo is not sufficient here since it takes some
// time for the FUSE layer in Android R+ to be ready after /storage/<UUID> has
// become a mountpoint.
func waitForARCVolumeMount(ctx context.Context, a *ARC, uuid string) error {
	// Regular expression that matches the output line for the mounted
	// volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	// Examples:
	//   1821167369 mounted 00000000000000000000000000000000DEADBEEF
	//   stub:18446744073709551614 mounted 0000000000000000000000000000CAFEF00D2019
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + uuid + `$`)

	testing.ContextLogf(ctx, "Waiting for the volume %s to be mounted in ARC", uuid)

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "sm command failed"))
		}
		lines := bytes.Split(out, []byte("\n"))
		for _, line := range lines {
			if re.Find(bytes.TrimSpace(line)) != nil {
				return nil
			}
		}
		return errors.New("the volume is not yet mounted")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// WaitForARCRemovableMediaVolumeMount waits for the removable media volume for
// testing to be mounted inside ARC.
func WaitForARCRemovableMediaVolumeMount(ctx context.Context, a *ARC) error {
	return waitForARCVolumeMount(ctx, a, RemovableMediaUUID)
}

// WaitForARCMyFilesVolumeMount waits for the MyFiles volume to be mounted
// inside ARC.
func WaitForARCMyFilesVolumeMount(ctx context.Context, a *ARC) error {
	return waitForARCVolumeMount(ctx, a, MyFilesUUID)
}

// WaitForARCMyFilesVolumeMountIfARCVMEnabled waits for the MyFiles volume to be
// mounted inside ARC only if ARCVM is enabled. Otherwise it just returns nil.
// This can be used in tests that write to or read from ARC's Download folder,
// because Downloads integraion in ARCVM depends on MyFiles mount.
func WaitForARCMyFilesVolumeMountIfARCVMEnabled(ctx context.Context, a *ARC) error {
	isARCVMEnabled, err := VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}
	if !isARCVMEnabled {
		return nil
	}
	return waitForARCVolumeMount(ctx, a, MyFilesUUID)
}
