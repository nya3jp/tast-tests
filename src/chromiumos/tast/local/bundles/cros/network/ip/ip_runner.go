// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// GetIPAddrFlags gets flags of the given interface.
func GetIPAddrFlags(ctx context.Context, iface string) ([]string, error) {
	reFlag := regexp.MustCompile(`<(.*)>`)
	out, err := testexec.CommandContext(ctx, "ip", "addr", "show", iface).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run \"ip addr show %s\"", iface)
	}
	match := reFlag.FindStringSubmatch(string(out))
	if match == nil {
		return nil, nil
	}
	return strings.Split(match[1], ","), nil
}

// upStateToStr returns "up" if up is true; otherwise, "down".
func upStateToStr(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

// PollIfaceUpDown polls for the interface being up/down.
func PollIfaceUpDown(ctx context.Context, iface string, expectUp bool, timeout time.Duration) error {
	// isIfaceUp returns true if the interface is up.
	isIfaceUp := func() (bool, error) {
		flags, err := GetIPAddrFlags(ctx, iface)
		if err == nil {
			for _, f := range flags {
				if f == "UP" {
					return true, nil
				}
			}
		}
		return false, err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isUp, err := isIfaceUp()
		if err != nil {
			return err
		}
		if isUp != expectUp {
			return errors.New("polling for iface " + upStateToStr(expectUp))
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(
			err, "failed to wait for interface %s to go %s",
			iface, upStateToStr(expectUp))
	}
	return nil
}

// SetIfaceUpDown sets the interface to up/down.
func SetIfaceUpDown(ctx context.Context, iface string, stateUp bool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := testexec.CommandContext(
		ctx, "ip", "link", "set", iface, upStateToStr(stateUp)).
		Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to run \"ip link set %s %s\"", iface, upStateToStr(stateUp))
	}
	return nil
}
