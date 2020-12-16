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

type ResultType float64

type msg struct {
	t      msgType
	err    error
	result []ResultType
}

type verifier struct {
	ctl  chan msg
	rev  chan msg
	fptr func() (ret ResultType, err error)
}

type workerState int

const (
	workerStateIdle workerState = iota
	workerStateRunning
	workerStateStopped
	workerStateError
)

func NewVerifier(ctx context.Context, fptr func() (ret ResultType, err error)) *verifier {
	ret := &verifier{}
	ret.ctl = make(chan msg)
	ret.rev = make(chan msg)
	ret.fptr = fptr
	go worker(ctx, ret)
	return ret
}

func (vf *verifier) StartJob() error {
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

func (vf *verifier) StopJob() ([]ResultType, error) {
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

func worker(ctx context.Context, vf *verifier) {
	state := workerStateIdle
	var results []ResultType
	var err error
	for state != workerStateStopped {
		select {
		case rcvMsg := <-vf.ctl:
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
		default:
			switch state {
			case workerStateIdle:
				// Wait some more to start.
				testing.Sleep(ctx, time.Second)
			case workerStateRunning:
				// TODO: handle error properly.
				ret, err := vf.fptr()
				if err != nil {
					testing.Sleep(ctx, 10*time.Second)
					state = workerStateStopped
				}
				results = append(results, ret)
			}
		}

	}
}
