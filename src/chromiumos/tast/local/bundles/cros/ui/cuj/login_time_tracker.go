// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

const (
	userLoginTimesPath = "/home/chronos/user/login-times"
	bootTimePattern    = `BootTime\.(LoginNewUser|Login): (([0-9]*[.])?[0-9]+)`
)

// LoginTimeTracker is a helper to collect the time used by chrome login.
type LoginTimeTracker struct {
	prefix string // Performance value prefix.

	loginType string  // Login type as read from login-times file: LoginNewUser or Login.
	loginTime float64 // Login time.
}

// NewLoginTimeTracker creates a new instance of LoginTimeTracker.
func NewLoginTimeTracker(ctx context.Context, metricPrefix string) (*LoginTimeTracker, error) {
	return &LoginTimeTracker{
		prefix: metricPrefix,
	}, nil
}

// Start starts the login time tracking. Currently it does nothing.
func (t *LoginTimeTracker) Start(ctx context.Context) error {
	return nil
}

// Stop stops login time tracking and sets the login time value.
func (t *LoginTimeTracker) Stop(ctx context.Context) error {
	f, err := os.Open(userLoginTimesPath)
	if os.IsNotExist(err) {
		// If the file doesn't exist, login time will not be recorded. This could happen
		// if testing is executed without logging in.
		return nil
	}
	if err != nil {
		return err
	}
	// Read first line.
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return err
	}

	// Match the pattern.
	matches := regexp.MustCompile(bootTimePattern).FindStringSubmatch(line)
	if matches == nil {
		return errors.New("boot time pattern is not found in the login-times file")
	}
	t.loginType = matches[1]
	loginTime, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to convert login time %q to float64", matches[2])
	}
	t.loginTime = loginTime

	return nil
}

// Record stores the collected data into pv for further processing.
func (t *LoginTimeTracker) Record(pv *perf.Values) {
	if t.loginTime != 0 {
		// Set User.LoginTime. Use default variant "summary" for backward compatibility.
		pv.Set(perf.Metric{
			Name:      t.prefix + "User.LoginTime",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
		}, float64(t.loginTime))
		// Set again login type as Variant, which could be useful for futher processing.
		pv.Set(perf.Metric{
			Name:      t.prefix + "User.LoginTime",
			Variant:   t.loginType,
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
		}, float64(t.loginTime))
	}
}
