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

// ECEvent is a struct to hold a wilco EC payload and the size of the payload.
// The size of the byte array for Payload will always be the maximum size
// for an EC event. The Size field in the struct represents how many bytes are
// valid in the Payload.
type ECEvent struct {
	Payload []byte
	Size    int
}

// ReadECEvent reads an EC event from the EC event device node. An ECEvent
// struct will be returned with the event payload and the size of the payload.
func ReadECEvent(ctx context.Context) (ECEvent, error) {
	ev := ECEvent{}
	ev.Payload = make([]byte, maxEventSize)

	f, err := os.OpenFile(eventReadPath, os.O_RDONLY, 0644)
	if err != nil {
		return ev, errors.Wrapf(err, "failed to open %v", eventReadPath)
	}
	defer f.Close()

	// Set the read deadline to the context deadline if it exists.
	if t, ok := ctx.Deadline(); ok {
		f.SetReadDeadline(t)
	}

	ev.Size, err = f.Read(ev.Payload)
	if os.IsTimeout(err) {
		return ev, err
	}
	if err != nil {
		return ev, errors.Wrapf(err, "failed to read from %v", eventReadPath)
	}

	return ev, nil
}

// ClearECEventQueue will read all of the currently available events in the wilco
// EC device node queue. These events are discarded.
func ClearECEventQueue(ctx context.Context) error {
	for {
		readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(100*time.Millisecond))
		defer cancel()
		_, err := ReadECEvent(readCtx)
		// If we have timedout reading the file, we have drained the queue.
		if os.IsTimeout(err) {
			break
		}
		if err != nil {
			return errors.Wrap(err, "unable to read EC event file")
		}
	}
	return nil
}
