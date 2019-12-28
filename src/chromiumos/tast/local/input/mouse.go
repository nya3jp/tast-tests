// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	"chromiumos/tast/testing"
)

// MouseEventWriter supports injecting events into a mouse device.
type MouseEventWriter struct {
	rw   *RawEventWriter
	virt *os.File
	dev  string
	ctx  context.Context
}

var nextVirtMouseNum = 1 // appended to virtual mouse device name

// Mouse creates a virtual mouse device and returns an EventWriter that injects events into it.
func Mouse(ctx context.Context) (*MouseEventWriter, error) {
	mw := &MouseEventWriter{ctx: ctx}

	name := fmt.Sprintf("Tast virtual mouse %d.%d", os.Getpid(), nextVirtMouseNum)
	nextVirtMouseNum++
	testing.ContextLogf(ctx, "Creating virtual mouse device %q", name)

	const usbBus = 0x3 // BUS_USB from input.h
	var err error
	var evTypes uint32 = 1<<EV_KEY | 1<<EV_REL
	if mw.dev, mw.virt, err = createVirtual(name, devID{usbBus, 0, 0, 0}, 0, evTypes,
		map[EventType]*big.Int{
			EV_KEY: makeBigIntFromEventCodes([]EventCode{BTN_LEFT}),
			EV_REL: makeBigIntFromEventCodes([]EventCode{REL_X, REL_Y}),
		}, nil); err != nil {
		return nil, err
	}

	// Sleep briefly to give Chrome and other processes time to see the new device.
	// TODO(crbug.com/1015264): Remove the hard-coded sleep.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Using virtual mouse device ", mw.dev)

	if mw.rw, err = Device(ctx, mw.dev); err != nil {
		mw.Close()
		return nil, err
	}

	return mw, nil
}

// Close closes the mouse device.
func (mw *MouseEventWriter) Close() error {
	var firstErr error
	if mw.rw != nil {
		firstErr = mw.rw.Close()
	}
	if mw.virt != nil {
		if err := mw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Move moves the mouse cursor. relX, relY are the new mouse position relative to the original mouse position in pixels.
// ChromeOS supports mouse acceleration, setting relX to large value like 100 may move the mouse for more than 100 pixels.
func (mw *MouseEventWriter) Move(relX, relY int32) error {
	if err := mw.rw.Event(EV_REL, REL_X, relX); err != nil {
		return err
	}
	if err := mw.rw.Event(EV_REL, REL_Y, relY); err != nil {
		return err
	}
	return mw.rw.Sync()
}

// MoveCursor moves the mouse cursor every interval for the specified duration.
func (mw *MouseEventWriter) MoveCursor(total time.Duration, interval time.Duration) error {
	loops := int(total / interval)

	// Reset the cursor to the top left.
	mw.Move(-10000, -10000)

	for i := 0; i < loops; i++ {
		// Moves mouse cursor back and forth diagonally.
		if i%100 < 50 {
			mw.Move(5, 5)
		} else {
			mw.Move(-5, -5)
		}
		// Sleeps briefly after each cursor move.
		if err := testing.Sleep(mw.ctx, interval); err != nil {
			return err
		}
	}

	return nil
}

// Click presses and releases the mouse left button.
func (mw *MouseEventWriter) Click() error {
	if err := mw.rw.Event(EV_KEY, BTN_LEFT, 1); err != nil {
		return err
	}
	if err := mw.rw.Sync(); err != nil {
		return err
	}
	if err := mw.rw.Event(EV_KEY, BTN_LEFT, 0); err != nil {
		return err
	}
	return mw.rw.Sync()
}
