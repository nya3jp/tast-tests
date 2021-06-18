// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package verifier

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ResultType defines abstract type of results type to handle.
type ResultType struct {
	Data      interface{}
	Timestamp time.Time
}

type eventType int

const (
	verifyStart eventType = iota
	verifyStartAck
	verifyStop
	verifyStopAck
	verifyFinish
	verifyTimeout
)

type event struct {
	t      eventType
	err    error
	result []ResultType
}

type workerState int

const (
	workerStateIdle workerState = iota
	workerStateRunning
	workerStateFinished
)

type eventStateTuple struct {
	state workerState
	event eventType
}

type transitionFptr func(ctx context.Context)

// Verifier structure with internal control data.
type Verifier struct {
	// Control channel, for sending commands.
	ctl chan event
	// Reverse channel, for returning ACKs and results.
	rev chan event
	// Function pointer to run in a loop.
	fptr func(ctx context.Context) (ret ResultType, err error)
	// State of the worker.
	state workerState
	// State transition table for handling states in the machine.
	transitionTable map[eventStateTuple]transitionFptr
	// Results storage.
	results []ResultType
}

// NewVerifier launches goroutine for verification and sets it up.
// The function should not take more than a minute, otherwise it may cause test flakiness.
// The function will run in a loop anyway.
func NewVerifier(ctx context.Context, fptr func(ctx context.Context) (ret ResultType, err error)) *Verifier {
	ret := &Verifier{}
	// We're using non-blocking channels to facilitate cleanup when the test fails, so we don't
	// hold one goroutine waiting for the other to receive the event (unless we explicitly want it).
	ret.ctl = make(chan event, 2)
	ret.rev = make(chan event, 2)
	ret.state = workerStateIdle
	ret.fptr = fptr
	ret.transitionTable = map[eventStateTuple]transitionFptr{
		{workerStateIdle, verifyStart}:      ret.startVerification,
		{workerStateIdle, verifyTimeout}:    ret.waitForEvent,
		{workerStateIdle, verifyFinish}:     ret.finishVerifier,
		{workerStateRunning, verifyStop}:    ret.stopVerification,
		{workerStateRunning, verifyTimeout}: ret.runVerificationRound,
		{workerStateRunning, verifyFinish}:  ret.finishVerifier,
	}
	go worker(ctx, ret)
	return ret
}

// StartJob starts new verification job. Wait for confirmation it has started.
func (vf *Verifier) StartJob() error {
	vf.ctl <- event{t: verifyStart}

	// We require ACK to proceed when starting a job.
	select {
	case ret, ok := <-vf.rev:
		if !ok {
			return errors.New("failed to receive response")
		}
		if ret.t != verifyStartAck {
			return errors.New("bad response received")
		}
		return ret.err
	case <-time.After(5 * time.Second):
		return errors.New("timed out waiting for a response")
	}
}

// StopJob stops running the job and return its results. It won't interrupt the job, it will simply
// stop looping the verification function once it returns.
func (vf *Verifier) StopJob() ([]ResultType, error) {
	vf.ctl <- event{t: verifyStop}
	select {
	case ret, ok := <-vf.rev:
		if !ok {
			return []ResultType{}, errors.New("failed to receive response")
		}
		if ret.t != verifyStopAck {
			return []ResultType{}, errors.New("bad response received")
		}
		return ret.result, ret.err
	case <-time.After(60 * time.Second):
		return []ResultType{}, errors.New("timed out waiting for a response")
	}
}

// Finish causes verification goroutine to exit.
func (vf *Verifier) Finish() {
	vf.ctl <- event{t: verifyFinish}
	close(vf.ctl)
}

func (vf *Verifier) startVerification(ctx context.Context) {
	vf.state = workerStateRunning
	testing.ContextLog(ctx, "Start Verification")
	vf.rev <- event{t: verifyStartAck}
}

func (vf *Verifier) stopVerification(ctx context.Context) {
	vf.state = workerStateIdle
	testing.ContextLog(ctx, "Stop Verification")
	vf.rev <- event{t: verifyStopAck, result: vf.results, err: nil}
	vf.results = nil
}

func (vf *Verifier) runVerificationRound(ctx context.Context) {
	ret, err := vf.fptr(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Error encountered during verification: ", err)
		// Simply: return from the goroutine.
		vf.state = workerStateFinished
	}
	vf.results = append(vf.results, ret)
}

func (vf *Verifier) waitForEvent(ctx context.Context) {
	// Block on reading. Should receive at least finish event upon shutdown.
	rcvEvt := <-vf.ctl
	vf.handleEvent(ctx, rcvEvt.t)
}

func (vf *Verifier) finishVerifier(ctx context.Context) {
	vf.state = workerStateFinished
}

func (vf *Verifier) handleEvent(ctx context.Context, evt eventType) {
	if f, ok := vf.transitionTable[eventStateTuple{vf.state, evt}]; !ok {
		testing.ContextLogf(ctx, "Bad state transition, state %d evt %d", vf.state, evt)
		// Simply: return from the goroutine.
		vf.state = workerStateFinished
	} else {
		f(ctx)
	}
}

func worker(ctx context.Context, vf *Verifier) {
	defer close(vf.rev)
	for vf.state != workerStateFinished {
		select {
		case rcvEvent := <-vf.ctl:
			vf.handleEvent(ctx, rcvEvent.t)
		default:
			vf.handleEvent(ctx, verifyTimeout)
		}
	}
}
