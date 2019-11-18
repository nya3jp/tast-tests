// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	eventTriggerPath = "/sys/kernel/debug/wilco_ec/test_event"
	eventReadPath    = "/dev/wilco_event0"
	maxEventSize     = 16
)

// TriggerECEvent writes data to the EC event trigger path and triggers a dummy
// EC event.
func TriggerECEvent() error {
	if err := ioutil.WriteFile(eventTriggerPath, []byte{0}, 0644); err != nil {
		return errors.Wrapf(err, "failed to write to %v", eventTriggerPath)
	}
	return nil
}

// ReadECEvent reads an EC event from the EC event device node. The event
// payload will be returned.
func ReadECEvent(ctx context.Context) ([]byte, error) {
	f, err := os.OpenFile(eventReadPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %v", eventReadPath)
	}
	defer f.Close()

	// Set the read deadline to the context deadline if it exists.
	if t, ok := ctx.Deadline(); ok {
		f.SetReadDeadline(t)
	}

	payload := make([]byte, maxEventSize)
	n, err := f.Read(payload)
	if os.IsTimeout(err) {
		return nil, err
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read from %v", eventReadPath)
	}

	return payload[:n], nil
}

// ClearECEventQueue will read all of the currently available events in the wilco
// EC device node queue. These events are discarded.
func ClearECEventQueue(ctx context.Context) error {
	for {
		readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
		defer cancel()
		event, err := ReadECEvent(readCtx)
		// If we have timedout reading the file, we have drained the queue.
		if os.IsTimeout(err) {
			break
		}
		if err != nil {
			return errors.Wrap(err, "unable to read EC event file")
		}

		testing.ContextLogf(ctx, "Removing stale event from EC event queue: [% #x]", event)
	}
	return nil
}
