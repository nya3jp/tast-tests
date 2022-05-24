// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jslog

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/testutil"
)

// fakeConsoleAPICalledClient is a fake implementation of ConsoleAPICalledClient.
type fakeConsoleAPICalledClient struct {
	ch chan *runtime.ConsoleAPICalledReply
}

func newFakeConsoleAPICalledClient() *fakeConsoleAPICalledClient {
	return &fakeConsoleAPICalledClient{make(chan *runtime.ConsoleAPICalledReply)}
}

func (c *fakeConsoleAPICalledClient) finish() {
	close(c.ch)
}

func (c *fakeConsoleAPICalledClient) send(r *runtime.ConsoleAPICalledReply) {
	c.ch <- r
}

func (c *fakeConsoleAPICalledClient) Recv() (*runtime.ConsoleAPICalledReply, error) {
	r := <-c.ch
	if r == nil {
		return nil, io.EOF
	}
	return r, nil
}

func (c *fakeConsoleAPICalledClient) Ready() <-chan struct{} {
	panic("not implemented")
}

func (c *fakeConsoleAPICalledClient) RecvMsg(m interface{}) error {
	panic("not implemented")
}

func (c *fakeConsoleAPICalledClient) Close() error {
	return nil
}

// verifyLog saves logs accumulated in m and compares the content with exp.
func verifyLog(t *testing.T, agg *Aggregator, exp string) {
	t.Helper()

	td := testutil.TempDir(t)
	defer os.RemoveAll(td)
	fn := filepath.Join(td, "jslog.txt")

	if err := agg.Save(fn); err != nil {
		t.Fatal("Failed to save JS logs: ", err)
	}

	data, err := ioutil.ReadFile(fn)
	if err != nil {
		t.Fatal("Failed to read JS logs: ", err)
	}

	log := string(data)
	if log != exp {
		t.Errorf("JS logs mismatch: got %q, want %q", log, exp)
	}
}

func TestLogger(t *testing.T) {
	agg := NewAggregator()
	defer agg.Close()

	ev1 := newFakeConsoleAPICalledClient()
	ev2 := newFakeConsoleAPICalledClient()
	w1 := agg.NewWorker("foo", "fooURL", ev1)
	w2 := agg.NewWorker("bar", "barURL", ev2)

	go func() {
		ts := time.Date(2018, 10, 26, 19, 20, 28, 0, time.Local)

		msg1 := "message1"
		ev1.send(&runtime.ConsoleAPICalledReply{
			Type:       "type",
			Args:       []runtime.RemoteObject{{Description: &msg1}},
			Timestamp:  runtime.Timestamp(ts.Unix() * 1000),
			StackTrace: nil,
		})
		ev1.finish()

		msg2 := "message2"
		ev2.send(&runtime.ConsoleAPICalledReply{
			Type:       "type",
			Args:       []runtime.RemoteObject{{Description: &msg2}},
			Timestamp:  runtime.Timestamp(ts.Unix() * 1000),
			StackTrace: nil,
		})
		ev2.finish()
	}()

	w1.Close()
	w2.Close()

	const exp = `================================================================== fooURL
2018-10-26 19:20:28 [type] message1

================================================================== barURL
2018-10-26 19:20:28 [type] message2

`
	verifyLog(t, agg, exp)

	// Log is cleared on save.
	verifyLog(t, agg, "")
}

func TestLogger_Empty(t *testing.T) {
	agg := NewAggregator()
	defer agg.Close()

	ev := newFakeConsoleAPICalledClient()
	w := agg.NewWorker("targetID", "initURL", ev)

	go ev.finish()
	w.Close()

	verifyLog(t, agg, "")
}

func TestLogger_ClearOnSave(t *testing.T) {
	agg := NewAggregator()
	defer agg.Close()

	ev := newFakeConsoleAPICalledClient()
	w := agg.NewWorker("foo", "fooURL", ev)

	go func() {
		ts := time.Date(2018, 10, 26, 19, 20, 28, 0, time.Local)
		msg := "message"
		ev.send(&runtime.ConsoleAPICalledReply{
			Type:       "type",
			Args:       []runtime.RemoteObject{{Description: &msg}},
			Timestamp:  runtime.Timestamp(ts.Unix() * 1000),
			StackTrace: nil,
		})
		ev.finish()
	}()

	w.Close()

	if err := agg.Save("/dev/null"); err != nil {
		t.Fatal("Failed to save: ", err)
	}

	// Log is cleared on save.
	verifyLog(t, agg, "")
}

func TestLogger_ErrorStackTrace(t *testing.T) {
	agg := NewAggregator()
	defer agg.Close()

	ev := newFakeConsoleAPICalledClient()
	w := agg.NewWorker("foo", "fooURL", ev)

	go func() {
		ts := time.Date(2018, 10, 26, 19, 20, 28, 0, time.Local)
		msg := "message"
		ev.send(&runtime.ConsoleAPICalledReply{
			Type:      "error",
			Args:      []runtime.RemoteObject{{Description: &msg}},
			Timestamp: runtime.Timestamp(ts.Unix() * 1000),
			StackTrace: &runtime.StackTrace{
				CallFrames: []runtime.CallFrame{
					{
						FunctionName: "foo",
						URL:          "chrome://foo",
						LineNumber:   11,
						ColumnNumber: 22,
					},
					{
						FunctionName: "bar",
						URL:          "chrome://bar",
						LineNumber:   33,
						ColumnNumber: 44,
					},
				},
			},
		})
		ev.finish()
	}()

	w.Close()

	const exp = `================================================================== fooURL
2018-10-26 19:20:28 [error] message
	at foo (chrome://foo [11:22])
	at bar (chrome://bar [33:44])

`
	verifyLog(t, agg, exp)
}

func TestLogger_InfoStackTrace(t *testing.T) {
	agg := NewAggregator()
	defer agg.Close()

	ev := newFakeConsoleAPICalledClient()
	w := agg.NewWorker("foo", "fooURL", ev)

	go func() {
		ts := time.Date(2018, 10, 26, 19, 20, 28, 0, time.Local)
		msg := "message"
		ev.send(&runtime.ConsoleAPICalledReply{
			Type:      "info", // info logs do not record stack traces
			Args:      []runtime.RemoteObject{{Description: &msg}},
			Timestamp: runtime.Timestamp(ts.Unix() * 1000),
			StackTrace: &runtime.StackTrace{
				CallFrames: []runtime.CallFrame{
					{
						FunctionName: "foo",
						URL:          "chrome://foo",
						LineNumber:   11,
						ColumnNumber: 22,
					},
					{
						FunctionName: "bar",
						URL:          "chrome://bar",
						LineNumber:   33,
						ColumnNumber: 44,
					},
				},
			},
		})
		ev.finish()
	}()

	w.Close()

	const exp = `================================================================== fooURL
2018-10-26 19:20:28 [info] message

`
	verifyLog(t, agg, exp)
}
