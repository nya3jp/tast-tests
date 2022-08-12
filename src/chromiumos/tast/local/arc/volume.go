// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
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

	// stubVolumeIDRegex is regex for volume IDs of StubVolumes (MyFiles,
	// removable media) in ARC.
	stubVolumeIDRegex = `(stub:)?[0-9]+`

	// sdCardVolumeIDRegex is regex for the volume ID of the sdcard volume in ARC.
	sdCardVolumeIDRegex = `emulated(;0)?`
)

// waitForARCVolumeStatusAndGetVolumeID waits for a volume of the given ID, state, and UUID
// appears inside ARC and returns the volume ID. Volume's ID, state, UUID should be expressed in regex.
// It looks up the output of "adb sm list-volumes" and returns the volume ID in the first line
// that matches the specified regex.
func waitForARCVolumeStatusAndGetVolumeID(ctx context.Context, a *ARC, id, state, uuid string) (string, error) {
	// Regular expression that matches the output line for the specified
	// volume. Each output line "adb sm list-volumes" is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	// Examples:
	//   emulated;0 mounted null
	//   1821167369 ejecting 00000000000000000000000000000000DEADBEEF
	//   stub:18446744073709551614 unmounted 0000000000000000000000000000CAFEF00D2019
	re := regexp.MustCompile(id + `\s+` + state + `\s+` + uuid)
	var volumeID string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "sm command failed"))
		}
		if matchedLine := re.Find(out); matchedLine != nil {
			volumeID = strings.Split(string(matchedLine), " ")[0]
			return nil
		}
		return errors.Errorf("no matching volume found for ID %q, state %q, UUID %q", id, state, uuid)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return "", err
	}
	return volumeID, nil
}

// waitForARCVolumeMount waits for a volume to be mounted in ARC using the sm
// command. Just checking mountinfo is not sufficient here since it takes some
// time for the FUSE layer in Android R+ to be ready after /storage/<UUID> has
// become a mountpoint.
func waitForARCVolumeMount(ctx context.Context, a *ARC, id, uuid string) error {
	_, err := waitForARCVolumeStatusAndGetVolumeID(ctx, a, id, "mounted", uuid)
	return err
}

// WaitForARCRemovableMediaVolumeMount waits for the removable media volume for
// testing to be mounted inside ARC.
func WaitForARCRemovableMediaVolumeMount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the removable volume to be mounted in ARC")

	return waitForARCVolumeMount(ctx, a, stubVolumeIDRegex, RemovableMediaUUID)
}

// WaitForARCMyFilesVolumeMount waits for the MyFiles volume to be mounted
// inside ARC.
func WaitForARCMyFilesVolumeMount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the MyFiles volume to be mounted in ARC")

	return waitForARCVolumeMount(ctx, a, stubVolumeIDRegex, MyFilesUUID)
}

// WaitForARCSDCardVolumeMount waits for the sdcard volume to be mounted
// inside ARC.
func WaitForARCSDCardVolumeMount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the sdcard volume to be mounted in ARC")

	return waitForARCVolumeMount(ctx, a, sdCardVolumeIDRegex, "null")
}

// waitForARCVolumeUnmount waits for a volume to be unmounted inside ARC.
func waitForARCVolumeUnmount(ctx context.Context, a *ARC, id, uuid string) error {
	_, err := waitForARCVolumeStatusAndGetVolumeID(ctx, a, id, "(unmounted|checking)", uuid)
	return err
}

// WaitForARCRemovableMediaVolumeUnmount waits for the removable media volume for
// testing to be unmounted inside ARC.
func WaitForARCRemovableMediaVolumeUnmount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the removable volume to be unmounted in ARC")

	return waitForARCVolumeUnmount(ctx, a, stubVolumeIDRegex, RemovableMediaUUID)
}

// WaitForARCMyFilesVolumeUnmount waits for the MyFiles volume to be unmounted
// inside ARC.
func WaitForARCMyFilesVolumeUnmount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the MyFiles volume to be unmounted in ARC")

	return waitForARCVolumeUnmount(ctx, a, stubVolumeIDRegex, MyFilesUUID)
}

// WaitForARCSDCardVolumeUnmount waits for the sdcard volume to be unmounted
// inside ARC.
func WaitForARCSDCardVolumeUnmount(ctx context.Context, a *ARC) error {
	testing.ContextLog(ctx, "Waiting for the sdcard volume to be mounted in ARC")

	return waitForARCVolumeUnmount(ctx, a, sdCardVolumeIDRegex, "null")
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
	return WaitForARCMyFilesVolumeMount(ctx, a)
}

// MyFilesVolumeID returns the volume ID of the MyFiles volume. It waits for
// the volume to be mounted if it is not mounted yet.
func MyFilesVolumeID(ctx context.Context, a *ARC) (string, error) {
	return waitForARCVolumeStatusAndGetVolumeID(ctx, a, stubVolumeIDRegex, "mounted", MyFilesUUID)
}

// SDCardVolumeID returns the volume ID of the sdcard volume
// (/storage/emulated/0). Although the volume ID itself is a constant, the
// function waits for the volume to be mounted if it is not mounted yet,
// so that the ID is guaranteed to be valid and usable inside ARC.
func SDCardVolumeID(ctx context.Context, a *ARC) (string, error) {
	return waitForARCVolumeStatusAndGetVolumeID(ctx, a, sdCardVolumeIDRegex, "mounted", "null")
}
