// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	metricsClientPath = "/usr/bin/metrics_client"
	// metricsClientArg: "-c: return exit status 0 if user consents to stats,""
	//                   "1 otherwise, in guest mode always return 1"
	metricsClientArg = "-c"
	// userConsents is the exit status if the user consents to stats.
	userConsents = 0
	// userDoesNotConsent is the exit status if the user does not consent to stats.
	userDoesNotConsent = 1
)

// HasConsent checks if the system has metrics consent.
func HasConsent(ctx context.Context) (bool, error) {
	// The C++ code reads the (possibly multiple) device policy files, and then
	// falls back to previous files if the latest file isn't valid, and then falls
	// back to enterprise enrollments and legacy consent files.
	// Rather than try to reproduce all that in Go, call a C++ program that runs
	// the exact same code that crash_reporter & crash_sender does.
	err := testexec.CommandContext(ctx, metricsClientPath, metricsClientArg).Run()
	code, ok := testexec.ExitCode(err)
	if !ok {
		return false, errors.Wrapf(err, "could not exec %s %s", metricsClientPath, metricsClientArg)
	}
	switch code {
	case userConsents:
		return true, nil
	case userDoesNotConsent:
		return false, nil
	default:
		return false, errors.Errorf("unexpected exit code from %s %s: %d", metricsClientPath, metricsClientArg, code)
	}
}
