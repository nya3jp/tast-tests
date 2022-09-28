// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gmscore provides set of util functions used to work with Gms Core.
package gmscore

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// WaitForGmsCorePersistentDone waits Gms Core persistent process finishes asynchronous background
// initialization flow. This is black-box process without exposing clear signals.
// vendor_system_native_boot.snapshotToken1 is the last preference set during this flow indicating
// vendor boot app is processed. This is not stable and may require updating with new GMS Core
// version. It is intended to be used for performance tests when heavy background flow may
// affect test results.
func WaitForGmsCorePersistentDone(ctx context.Context, a *arc.ARC, user string) error {
	testing.ContextLog(ctx, "Waiting for com.google.android.gms.persistent to become ready")

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to verify VM status")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	const prefsPath = "/data/data/com.google.android.gms/shared_prefs/platform_prefs.xml"
	hostPrefsPath := filepath.Join(rootCryptDir, "android-data", prefsPath)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cleanupFunc, err := arc.MountVirtioBlkDataDiskImageReadOnlyIfUsed(ctx, a, user)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer cleanupFunc(cleanupCtx)

		data, err := ioutil.ReadFile(hostPrefsPath)
		if err != nil {
			return err
		}

		if isVMEnabled {
			if !strings.Contains(string(data), "<string name=\"vendor_system_native_boot.snapshotToken1\">") {
				return errors.New("Not yet found")
			}
		}

		return nil
	}, &testing.PollOptions{Interval: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait GMS Core persistent done")
	}

	return nil
}
