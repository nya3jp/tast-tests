// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crastestclient

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// RequestFloopMask requests the flexible loopback device with the given mask
// returns the device id
func RequestFloopMask(ctx context.Context, mask int) (dev int, err error) {
	cmd := testexec.CommandContext(
		ctx,
		"cras_test_client",
		fmt.Sprintf("--request_floop_mask=%d", mask),
	)
	stdout, _, err := cmd.SeparatedOutput()
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`flexible loopback dev id: (\d+)`)
	m := re.FindSubmatch(stdout)
	if m == nil {
		return -1, errors.Errorf("output %q not matching %q", string(stdout), re)
	}
	return strconv.Atoi(string(m[1]))
}
