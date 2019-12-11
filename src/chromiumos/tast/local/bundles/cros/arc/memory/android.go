// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file contains helper functions to allocate memory on Android.

package memory

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// broadcastResultRegexp matches the result from an Android Activity Manager broadcast.
var broadcastResultRegexp *regexp.Regexp

func init() {
	broadcastResultRegexp = regexp.MustCompile(`Broadcast completed: result=(-?[0-9]+)(, data="(.*)")?`)
}

// AndroidAllocator helps allocate memory on Android.
type AndroidAllocator struct {
	*arc.ARC
}

// NewAndroidAllocator creates a helper for allocating Android memory.
func NewAndroidAllocator(a *arc.ARC) AndroidAllocator {
	return AndroidAllocator{a}
}

// broadcast sends an Android broadcast Intent to the ArcMemoryAllocatorTest app.
// Returns the data from the broadcast response.
func (a AndroidAllocator) broadcast(ctx context.Context, action string, extras ...string) ([]byte, error) {
	const actionPrefix = "org.chromium.arc.testapp.memoryallocator."

	args := []string{"broadcast", "-a", actionPrefix + action}
	output, err := a.Command(ctx, "am", append(args, extras...)...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send broadcast intent to ArcMemoryAllocatorTest app")
	}
	m := broadcastResultRegexp.FindSubmatch(output)
	if m == nil {
		return nil, errors.Errorf("unable to parse broadcast result %q", output)
	}
	// Expect Activity.RESULT_OK == -1
	if string(m[1]) != "-1" {
		return nil, errors.Errorf("broadcast failed, status = %s, %q", m[1], output)
	}
	if string(m[2]) == "" {
		// No data returned
		return nil, nil
	}
	return m[3], nil
}

// jsonBroadcast sends an Android broadcast Intent to the
// ArcMemoryAllocatorTest app, and parses the data returned as JSON into the
// passed 'v' parameter.
func (a AndroidAllocator) jsonBroadcast(ctx context.Context, v interface{}, action string, extras ...string) error {
	res, err := a.broadcast(ctx, action, extras...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(res, v); err != nil {
		return errors.Wrapf(err, "failed to parse result %q", res)
	}
	return nil
}

// Prepare the ArcMemoryAllocatorTest app for a test.
// Installs the app, and also turns off SELinux enforce (on ARC++) so that the
// app can read the available memory counter in sysfs.
// Returns a function that uninstalls the app, and turns SELinux enforce back
// on if it was turned off.
func (a AndroidAllocator) Prepare(ctx context.Context, dataPathGetter func(string) string) (func() error, error) {
	const (
		activity = "org.chromium.arc.testapp.memoryallocator/.MainActivity"
		apk      = "ArcMemoryAllocatorTest.apk"
		pkg      = "org.chromium.arc.testapp.memoryallocator"
	)

	if err := a.Install(ctx, dataPathGetter(apk)); err != nil {
		return nil, errors.Wrap(err, "failed to install ArcMemoryAllocatorTest app")
	}
	cleanup1 := func() error {
		if err := a.Uninstall(ctx, pkg); err != nil {
			return errors.Wrap(err, "failed to uninstall ArcMemoryAllocatorTest app")
		}
		return nil
	}

	if err := a.Command(ctx, "am", "start", "-W", activity).Run(testexec.DumpLogOnError); err != nil {
		cleanup1()
		return nil, errors.Wrap(err, "failed to start ArcMemoryAllocatorTest app")
	}

	// Disable SELinux enforce, so that we can read memory counters from the guest in ARC++
	if vmEnabled, err := arc.VMEnabled(); vmEnabled || err != nil {
		if err != nil {
			cleanup1()
			return nil, errors.Wrap(err, "failed to check if VM is enabled")
		}
		// Counters aren't blocked by SELinux in ARCVM
		return cleanup1, nil
	}
	output, err := testexec.CommandContext(ctx, "getenforce").Output(testexec.DumpLogOnError)
	if err != nil {
		cleanup1()
		return nil, errors.Wrap(err, "failed to read SELinux enforcement")
	}
	if strings.TrimSpace(string(output)) != "Enforcing" {
		cleanup1()
		return nil, errors.Errorf("selinux not Enforcing %s", output)
	}
	if err := testexec.CommandContext(ctx, "setenforce", "0").Run(testexec.DumpLogOnError); err != nil {
		cleanup1()
		return nil, errors.Wrap(err, "failed to disable SELinux enforcement")
	}
	cleanup2 := func() error {
		if err := cleanup1(); err != nil {
			return err
		}
		if err := testexec.CommandContext(ctx, "setenforce", "1").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to reenable SELinux enforcement")
		}
		return nil
	}
	return cleanup2, nil
}

// msToString converts a time.Duration to a string containing the duration in
// milliseconds. This should be removed after Go 1.3, since these helpers are
// built in.
func msToString(d time.Duration) string {
	return strconv.FormatInt(int64(d/time.Millisecond), 10)
}

// AllocateUntil allocates memory until available memory is at the passed
// margin.  To allow the system to stabalize, it will try attempts times,
// waiting attemptInterval duration between each attempt.
// If too much memory has been allocated, then the extra is freed between
// attempts to avoid overshooting the margin.
// Returns the allocated memory at every attempt.
func (a AndroidAllocator) AllocateUntil(
	ctx context.Context,
	attemptInterval time.Duration,
	attempts int,
	margin int,
) ([]int, error) {
	if _, err := a.broadcast(
		ctx,
		"ALLOC_UNTIL",
		"--ei", "attempt_timeout", msToString(attemptInterval),
		"--ei", "attempts", strconv.Itoa(attempts),
		"--ei", "margin", strconv.Itoa(margin),
	); err != nil {
		return nil, errors.Wrap(err, "failed to request allocation")
	}

	for done := false; !done; {
		testing.Sleep(ctx, 1*time.Second)
		if err := a.jsonBroadcast(ctx, &done, "ALLOC_DONE"); err != nil {
			return nil, errors.Wrap(err, "failed to poll ALLOC_DONE")
		}
	}
	var allocated []int
	if err := a.jsonBroadcast(ctx, &allocated, "ALLOC_ATTEMPTS"); err != nil {
		return nil, errors.Wrap(err, "failed to read alloc attempts")
	}
	if len(allocated) != attempts {
		return nil, errors.Errorf("wrong number of attempts returned from app, expected %d, got %d", attempts, len(allocated))
	}
	return allocated, nil
}

// FreeAll frees all allocated Android memory.
func (a AndroidAllocator) FreeAll(ctx context.Context) error {
	if _, err := a.broadcast(ctx, "FREE_ALL"); err != nil {
		return errors.Wrap(err, "failed to free")
	}
	return nil
}
