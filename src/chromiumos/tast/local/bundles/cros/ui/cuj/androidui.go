// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
)

// OpenAppAndGetStartTime launches a new activity, starts it and records start time.
// The caller must close the returned activity when the test is done.
func OpenAppAndGetStartTime(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC,
	pkgName, appName, startActivity string) (time.Duration, *arc.Activity, error) {
	act, err := arc.NewActivity(a, pkgName, startActivity)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "failed to create new activity for %s", startActivity)
	}
	startTime := time.Now()
	// Start() will invoke "am start" and waits for the app to be visible on the Chrome side.
	if err := act.Start(ctx, tconn); err != nil {
		return 0, nil, errors.Wrapf(err, "failed to start %s", appName)
	}

	return time.Since(startTime), act, nil
}

// FindAndClick finds and clicks Android ui object.
// Deprecated: Use FindAndClickAction.
func FindAndClick(ctx context.Context, obj *ui.Object, timeout time.Duration) error {
	if err := obj.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the target object")
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the target object")
	}
	return nil
}

// FindAndClickAction returns an action function which finds and clicks Android ui object.
func FindAndClickAction(obj *ui.Object, waitTime time.Duration) action.Action {
	return func(ctx context.Context) error {
		return FindAndClick(ctx, obj, waitTime)
	}
}

// ClickIfExist clicks the UI object if it exists. If the object cannot be found, nil will be returned.
// Deprecated: Use ClickIfExistAction.
func ClickIfExist(ctx context.Context, obj *ui.Object, timeout time.Duration) error {
	if err := obj.WaitForExists(ctx, timeout); err != nil {
		return nil
	}
	return obj.Click(ctx)
}

// ClickIfExistAction returns an action function which clicks the UI object if it exists.
func ClickIfExistAction(obj *ui.Object, waitTime time.Duration) action.Action {
	return func(ctx context.Context) error {
		return ClickIfExist(ctx, obj, waitTime)
	}
}

// DumpArcUITreeOnError dumps arc tree to 'arc_uidump.xml', when the test fails.
func DumpArcUITreeOnError(ctx context.Context, outDir string, hasError func() bool) error {
	if !hasError() {
		return nil
	}

	a, err := arc.New(ctx, outDir)
	if err != nil {
		return errors.Wrap(err, "failed to setup arc")
	}
	defer a.Close(ctx)

	if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to dump arc UI")
	}

	dir := filepath.Join(outDir, "faillog")
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	file := filepath.Join(dir, "arc_uidump.xml")
	if err := a.PullFile(ctx, "/sdcard/window_dump.xml", file); err != nil {
		return errors.Wrap(err, "failed to pull UI dump to outDir")
	}

	return nil
}
