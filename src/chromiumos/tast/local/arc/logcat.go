// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"

	"chromiumos/tast/local/android/adb"
)

// RegexpPred returns a function to be passed to WaitForLogcat that returns true if a given regexp is matched in that line.
func RegexpPred(exp *regexp.Regexp) func(string) bool {
	return adb.RegexpPred(exp)
}

// WaitForLogcat keeps scanning logcat. The function pred is called on the logcat contents line by line. This function returns successfully if pred returns true. If pred never returns true, this function returns an error as soon as the context is done.
// An optional quitFunc will be polled at regular interval, which can be used to break early if for example the activity which is supposed to print the exp has crashed
func (a *ARC) WaitForLogcat(ctx context.Context, pred func(string) bool, quitFunc ...func() bool) error {
	return a.device.WaitForLogcat(ctx, pred, quitFunc...)
}
