// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type msgType int

const (
	measureStart msgType = iota
	measureStartAck
	measureStop
	measureStopAck
)

type ResultType struct {
	Data      interface{}
	Timestamp time.Time
}

type msg struct {
	t      msgType
	err    error
	result []ResultType
}

type workerState int

type Verifier struct {
	ctl  chan msg
	rev  chan msg
	fptr func() (ret ResultType, err error)
}

const (
	workerStateIdle workerState = iota
	workerStateRunning
	workerStateStopped
	workerStateError
)

func NewVerifier(ctx context.Context, fptr func() (ret ResultType, err error)) *Verifier {
	ret := &Verifier{}
	ret.ctl = make(chan msg)
	ret.rev = make(chan msg)
	ret.fptr = fptr
	go worker(ctx, ret)
	return ret
}

func (vf *Verifier) StartJob() error {
	vf.ctl <- msg{t: measureStart}
	ret, ok := <-vf.rev
	if !ok {
		return errors.New("Failed to receive response.")
	}
	if ret.t != measureStartAck {
		return errors.New("Bad response received.")
	}
	return ret.err
}

func (vf *Verifier) StopJob() ([]ResultType, error) {
	vf.ctl <- msg{t: measureStop}
	ret, ok := <-vf.rev
	if !ok {
		return []ResultType{}, errors.New("Failed to receive response")
	}
	if ret.t != measureStopAck {
		return []ResultType{}, errors.New("Bad response received.")
	}
	return ret.result, ret.err
}

func worker(ctx context.Context, vf *Verifier) {
	state := workerStateIdle
	var results []ResultType
	var err error
	handleMsg := func(rcvMsg msg) {
		switch rcvMsg.t {
		case measureStart:
			state = workerStateRunning
			testing.ContextLog(ctx, "Start")
			vf.rev <- msg{t: measureStartAck}
		case measureStop:
			state = workerStateStopped
			testing.ContextLog(ctx, "Stop")
			vf.rev <- msg{t: measureStopAck, result: results, err: err}
		}
	}
	for state != workerStateStopped {
		if state == workerStateIdle {
			// Block on reading
			rcvMsg := <-vf.ctl
			handleMsg(rcvMsg)
			continue
		}
		select {
		case rcvMsg := <-vf.ctl:
			handleMsg(rcvMsg)
		default:
			switch state {
			case workerStateRunning:
				// TODO: handle error properly.
				ret, err := vf.fptr()
				if err != nil {
					vf.rev <- msg{t: measureStopAck, result: results, err: err}
					state = workerStateStopped
				}
				results = append(results, ret)
			}
		}

	}
}
