// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
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
)

// waitForARCVolumeStatusChange waits for the status of a volume to be changed (e.g. unmounted / mounted)
// in ARC using the sm command.
func waitForARCVolumeStatusChange(ctx context.Context, a *ARC, re *regexp.Regexp) error {
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

// waitForARCVolumeMount waits for a volume to be mounted in ARC using the sm
// command. Just checking mountinfo is not sufficient here since it takes some
// time for the FUSE layer in Android R+ to be ready after /storage/<UUID> has
// become a mountpo.
func waitForARCVolumeMount(ctx context.Context, a *ARC, uuid string) error {
	// Regular expression that matches the output line for the mounted
	// volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	// Examples:
	//   1821167369 mounted 00000000000000000000000000000000DEADBEEF
	//   stub:18446744073709551614 mounted 0000000000000000000000000000CAFEF00D2019
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + uuid + `$`)

	testing.ContextLogf(ctx, "Waiting for the volume %s to be mounted in ARC", uuid)

	return waitForARCVolumeStatusChange(ctx, a, re)
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

// WaitForARCSdcardVolumeMount waits for the sdcard volume to be mounted
// inside ARC.
func WaitForARCSdcardVolumeMount(ctx context.Context, a *ARC) error {
	// Regular expression that matches the output line for the mounted
	// volume. Each output line of the sm command is of the form:
	// emulated;0<space(s)><mount status>
	re := regexp.MustCompile(`emulated;0\s+mounted`)
	return waitForARCVolumeStatusChange(ctx, a, re)
}

// waitForARCVolumeUnmount waits for a volume to be unmounted inside ARC.
func waitForARCVolumeUnmount(ctx context.Context, a *ARC, uuid string) error {
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+unmounted\s+` + uuid + `$`)

	testing.ContextLogf(ctx, "Waiting for the volume %s to be unmounted in ARC", uuid)

	return waitForARCVolumeStatusChange(ctx, a, re)
}

// WaitForARCRemovableMediaVolumeUnmount waits for the removable media volume for
// testing to be unmounted inside ARC.
func WaitForARCRemovableMediaVolumeUnmount(ctx context.Context, a *ARC) error {
	return waitForARCVolumeUnmount(ctx, a, RemovableMediaUUID)
}

// WaitForARCMyFilesVolumeUnmount waits for the MyFiles volume to be unmounted
// inside ARC.
func WaitForARCMyFilesVolumeUnmount(ctx context.Context, a *ARC) error {
	return waitForARCVolumeUnmount(ctx, a, MyFilesUUID)
}

// WaitForARCSdcardVolumeUnmount waits for the sdcard volume to be unmounted
// inside ARC.
func WaitForARCSdcardVolumeUnmount(ctx context.Context, a *ARC) error {
	re := regexp.MustCompile(`emulated;0\s+unmounted`)
	return waitForARCVolumeStatusChange(ctx, a, re)
}

// WaitForARCMyFilesVolumeMountIfARCVMEnabled waits for the MyFiles volume to be
// mounted inside ARC only if ARCVM is enabled. Otherwise it just returns nil.
// This can be used in tests that write to or read from ARC's Download folder,
// because Downloads egraion in ARCVM depends on MyFiles mount.
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

// SdcardVolumeID returns the volume ID of the sdcard volume
// (/storage/emulated/0). Although the volume ID itself is a constant, the
// function waits for the volume to be mounted if it is not mounted yet,
// so that the ID is guaranteed to be valid and usable inside ARC.
func SdcardVolumeID(ctx context.Context, a *ARC) (string, error) {
	if err := WaitForARCSdcardVolumeMount(ctx, a); err != nil {
		return "", err
	}
	return "emulated;0", nil
}

// MyFilesVolumeID returns the volume ID of the MyFiles volume. It waits for
// the volume to be mounted if it is not mounted yet.
func MyFilesVolumeID(ctx context.Context, a *ARC) (string, error) {
	// Regular expression that matches the output line for the mounted
	// MyFiles volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + MyFilesUUID + `$`)
	var volumeID string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		lines := bytes.Split(out, []byte("\n"))
		for _, line := range lines {
			if volumeIDLine := re.Find(bytes.TrimSpace(line)); volumeIDLine != nil {
				splitVolumeIDLine := strings.Split(string(volumeIDLine), " ")
				volumeID = splitVolumeIDLine[0]
				return nil
			}
		}
		return errors.New("MyFiles volume is not yet mounted")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return "", errors.Wrap(err, "failed to find myfiles volume id")
	}
	return volumeID, nil
}
