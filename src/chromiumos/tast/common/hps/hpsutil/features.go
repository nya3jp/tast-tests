// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// PersonPresentPageArchiveFilename the file name for persen present page.
	PersonPresentPageArchiveFilename = "person-present-page.tar.xz"

	// WaitNOpsBeforeStart How many ops to wait before starting to test after starting Chrome.
	// Since HPS has auto-exposure give it some time to figure the exposure.
	WaitNOpsBeforeStart = 10

	// WaitNOpsBeforeExpectingPresenceChange How many ops to wait when changing from no person to person in frame or
	// vice versa.
	WaitNOpsBeforeExpectingPresenceChange = 1

	// GetNOpsToVerifyPresenceWorks How many ops to run to ensure that presence model works reliably.
	GetNOpsToVerifyPresenceWorks = 10
)

// WaitForNPresenceOps waits for the num of ops to be run.
func WaitForNPresenceOps(hctx *HpsContext, numOps int, feature string) ([]int, error) {
	var result []int
	ctx := hctx.Ctx
	reg := "8"
	if feature == "1" {
		reg = "9"
	}

	testing.ContextLog(ctx, "waitForNPresenceOps ", numOps)
	start, err := GetNumberOfPresenceOps(hctx)
	if err != nil {
		return result, errors.Wrap(err, "waitForNewPresence: Failed to get initial number of operations")
	}
	last := start
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		current, err := GetNumberOfPresenceOps(hctx)
		if err != nil {
			return err
		}
		if int(current) > int(start)+numOps {
			return nil
		}
		if current > last {
			last = current
			presence, err := GetPresenceResult(hctx, reg)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get presence result"))
			}
			testing.ContextLog(ctx, "Got presence result: ", presence)
			result = append(result, presence)
		}
		return errors.Errorf("started with %q, haven't finished with %q", start, current)
	}, &testing.PollOptions{
		Interval: 50 * time.Millisecond,
		Timeout:  1 * time.Duration(numOps) * time.Minute,
	}); err != nil {
		return result, errors.Wrap(err, "failed wait for new inference")
	}
	if len(result) != int(numOps) {
		return result, errors.Errorf("Wrong number of presence results: Expected %q Got %q (%q)", numOps, len(result), result)
	}
	return result, nil
}

// EnablePresence power-cycles the HPS devboard and enables presence detection.
// TODO: fail the test if reg6 is not 0x0000 at any point in time?
// It takes ~2 minutes to enable presence after reset.
// 0 -- enable detecting one person
// 1 -- enable second person alert
func EnablePresence(hctx *HpsContext, feature string) (time.Duration, error) {
	if err := hctx.PowerCycle(); err != nil {
		return 0, err
	}
	status := `\b0x0001\b`
	if feature == "1" {
		status = `\b0x0002\b`
	}

	start := time.Now()

	if err := RunHpsTool(hctx, "cmd", "launch"); err != nil {
		return 0, err
	}

	if err := pollStatus(hctx, "2", `\bkStage1\b`); err != nil {
		return 0, err
	}

	if err := RunHpsTool(hctx, "cmd", "appl"); err != nil {
		return 0, err
	}

	if err := pollStatus(hctx, "2", `\bkAppl\b`); err != nil {
		return 0, err
	}

	if err := RunHpsTool(hctx, "enable", feature); err != nil {
		return 0, err
	}

	if err := pollStatus(hctx, "7", status); err != nil {
		return 0, err
	}

	return time.Now().Sub(start), nil
}

func pollStatus(hctx *HpsContext, register, pattern string) error {
	testing.ContextLog(hctx.Ctx, "Polling hps status register ", register, " for '", pattern, "'")
	regex := regexp.MustCompile(pattern)
	args := []string{"hps", hctx.Device, "status", register}

	if err := testing.Poll(hctx.Ctx, func(ctx context.Context) error {
		var output []byte
		var err error

		if hctx.DutConn != nil {
			output, err = hctx.DutConn.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		} else {
			output, err = testexec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		}
		if err != nil {
			return err
		}
		matched := regex.MatchString(string(output))
		if matched {
			return nil
		}
		return errors.Errorf("%q not found in %q", pattern, string(output))
	}, &testing.PollOptions{
		Interval: 100 * time.Millisecond,
		Timeout:  5 * time.Minute,
	}); err != nil {
		return errors.Wrap(err, "failed wait for kStage1")
	}
	return nil
}
