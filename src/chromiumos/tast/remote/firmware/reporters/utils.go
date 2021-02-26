// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"strings"
)

// DoAllPathsExist reports whether all given paths exist on the DUT.
func (r *Reporter) DoAllPathsExist(ctx context.Context, paths []string) (bool, error) {
	out, err := r.CombinedOutput(ctx, "file", append([]string{"-E"}, paths...)...)
	if err == nil {
		return true, nil
	}
	if strings.Contains(out, "No such file or directory") {
		return false, nil
	}
	return false, err
}
