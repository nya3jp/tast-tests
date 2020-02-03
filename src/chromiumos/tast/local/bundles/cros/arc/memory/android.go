// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file contains helper functions to allocate memory on Android.

package memory

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// broadcastResultRegexp matches the result from an Android Activity Manager broadcast.
var broadcastResultRegexp = regexp.MustCompile(`Broadcast completed: result=(-?[0-9]+)(, data="(.*)")?`)

// AndroidAllocator helps allocate memory on Android.
type AndroidAllocator struct {
	a *arc.ARC
}

// NewAndroidAllocator creates a helper for allocating Android memory.
func NewAndroidAllocator(a *arc.ARC) *AndroidAllocator {
	return &AndroidAllocator{a}
}

// broadcast sends an Android broadcast Intent to the ArcMemoryAllocatorTest app.
// Returns the data from the broadcast response.
func (a *AndroidAllocator) broadcast(ctx context.Context, action string, extras ...string) ([]byte, error) {
	const actionPrefix = "org.chromium.arc.testapp.memoryallocator."

	args := []string{"broadcast", "-a", actionPrefix + action}
	output, err := a.a.Command(ctx, "am", append(args, extras...)...).Output(testexec.DumpLogOnError)
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
func (a *AndroidAllocator) jsonBroadcast(ctx context.Context, v interface{}, action string, extras ...string) error {
	res, err := a.broadcast(ctx, action, extras...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(res, v); err != nil {
		return errors.Wrapf(err, "failed to parse result %q", res)
	}
	return nil
}

// AndroidSetup adds setup.Actions to the passed setup.Setup that installs
// and starts the Android allocation test App.
func AndroidSetup(ctx context.Context, a *arc.ARC, s *setup.Setup, dataPathGetter func(string) string) {
	const (
		activity = ".MainActivity"
		apk      = "ArcMemoryAllocatorTest.apk"
		pkg      = "org.chromium.arc.testapp.memoryallocator"
	)
	apkDataPath := dataPathGetter(apk)
	s.Append(arc.DisableSELinux(ctx, true))
	s.Append(arc.InstallApp(ctx, a, apkDataPath, pkg))
	s.Append(arc.StartActivity(ctx, a, pkg, activity))
}

// msToString converts a time.Duration to a string containing the duration in
// milliseconds. This should be removed after Go 1.13, since these helpers are
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
func (a *AndroidAllocator) AllocateUntil(
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
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return nil, errors.Wrap(err, "failed to sleep between allocation attempts")
		}
		if err := a.jsonBroadcast(ctx, &done, "ALLOC_DONE"); err != nil {
			return nil, errors.Wrap(err, "failed to poll ALLOC_DONE")
		}
	}
	var allocated []int
	if err := a.jsonBroadcast(ctx, &allocated, "ALLOC_ATTEMPTS"); err != nil {
		return nil, errors.Wrap(err, "failed to read alloc attempts")
	}
	if len(allocated) != attempts {
		return nil, errors.Errorf("wrong number of attempts returned from app, got %d, expected %d", len(allocated), attempts)
	}
	return allocated, nil
}

// FreeAll frees all allocated Android memory.
func (a *AndroidAllocator) FreeAll(ctx context.Context) error {
	if _, err := a.broadcast(ctx, "FREE_ALL"); err != nil {
		return errors.Wrap(err, "failed to free")
	}
	return nil
}
