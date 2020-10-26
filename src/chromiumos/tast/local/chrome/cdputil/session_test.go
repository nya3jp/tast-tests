// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cdputil

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/protocol/target"
	"github.com/mafredri/cdp/rpcc"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

func TestReadDebuggingPort(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	for _, tc := range []struct {
		name, data string
		port       int // expected port or -1 if error is expected
	}{
		{"full", "56245\n/devtools/browser/01db187e-2e2a-42c5-833e-bd4dbea9e313", 56245},
		{"oneline", "123", 123},
		{"garbage", "foo", -1},
		{"empty", "", -1},
	} {
		p := filepath.Join(td, tc.name)
		if err := ioutil.WriteFile(p, []byte(tc.data), 0644); err != nil {
			t.Fatal(err)
		}
		port, err := readDebuggingPort(p)
		if tc.port == -1 {
			if err == nil {
				t.Errorf("readDebuggingPort(%q) (data %q) didn't return expected error", tc.name, tc.data)
			}
		} else {
			if err != nil {
				t.Errorf("readDebuggingPort(%q) (data %q) returned error: %v", tc.name, tc.data, err)
			} else if port != tc.port {
				t.Errorf("readDebuggingPort(%q) (data %q) = %d; want %d", tc.name, tc.data, port, tc.port)
			}
		}
	}

	if _, err := readDebuggingPort(filepath.Join(td, "missing")); err == nil {
		t.Error("readDebuggingPort didn't return expected error for missing file")
	}
}

type fakeConn struct {
	done   chan struct{}
	cancel context.CancelFunc
}

func (dc *fakeConn) Read(p []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (dc *fakeConn) Write(p []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (dc *fakeConn) Close() error {
	dc.cancel()
	return nil
}

func newFakeConn(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	dc := &fakeConn{done: make(chan struct{}), cancel: cancel}
	go func() {
		select {
		case <-ctx.Done():
			close(dc.done)
		}
	}()
	return dc, nil
}

type fakeCodec struct {
	dconn         *fakeConn
	ch            chan uint64
	requestedArgs []interface{}
	responses     []interface{}
}

func (dc *fakeCodec) ReadResponse(resp *rpcc.Response) error {
	var id uint64
	select {
	case id = <-dc.ch:
		break
	case <-dc.dconn.done:
		return errors.New("already closed")
	}

	if len(dc.responses) == 0 {
		return errors.New("no expected responses found")
	}
	nextResponse := dc.responses[0]
	dc.responses = dc.responses[1:]
	var err error
	if resp.Result, err = json.Marshal(nextResponse); err != nil {
		return err
	}
	resp.ID = id
	return nil
}

func (dc *fakeCodec) WriteRequest(req *rpcc.Request) error {
	dc.requestedArgs = append(dc.requestedArgs, req.Args)
	select {
	case dc.ch <- req.ID:
		return nil
	case <-dc.dconn.done:
		return errors.New("already closed")
	}
}

func (dc *fakeCodec) AppendExpectedResponse(resp interface{}) {
	dc.responses = append(dc.responses, resp)
}

func newFakeSession() (sess *Session, dc *fakeCodec, err error) {
	dc = &fakeCodec{ch: make(chan uint64)}
	conn, err := rpcc.Dial("fake", rpcc.WithDialer(newFakeConn), rpcc.WithCodec(func(conn io.ReadWriter) rpcc.Codec {
		dc.dconn = conn.(*fakeConn)
		return dc
	}))
	if err != nil {
		return nil, nil, err
	}
	return &Session{client: cdp.NewClient(conn)}, dc, nil
}

func TestCreateTarget(t *testing.T) {
	for _, c := range []struct {
		name               string
		opts               []CreateTargetOption
		expectedNewWindow  bool
		expectedBackground bool
	}{
		{
			name: "no-options",
		},
		{
			name:              "new-window",
			opts:              []CreateTargetOption{WithNewWindow()},
			expectedNewWindow: true,
		},
		{
			name:               "background",
			opts:               []CreateTargetOption{WithBackground()},
			expectedBackground: true,
		},
		{
			name:               "both",
			opts:               []CreateTargetOption{WithNewWindow(), WithBackground()},
			expectedNewWindow:  true,
			expectedBackground: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sess, dc, err := newFakeSession()
			if err != nil {
				t.Fatal("Failed to dial: ", err)
			}
			dc.AppendExpectedResponse(&target.CreateTargetReply{TargetID: "fake-id"})
			id, err := sess.CreateTarget(ctx, "about:blank", c.opts...)
			if err != nil {
				t.Fatal("Failed to call CreateTarget: ", err)
			}
			if id != "fake-id" {
				t.Errorf("Expected fake-id, got %s", id)
			}
			if len(dc.requestedArgs) != 1 {
				t.Fatalf("Expected number of requests 1, got %d", len(dc.requestedArgs))
			}
			lastArgs, ok := dc.requestedArgs[0].(*target.CreateTargetArgs)
			if !ok {
				t.Fatalf("The lastRequestArgs expected to be a CreateTargetArgs instance, got %v", lastArgs)
			}
			gotNewWindow := false
			if lastArgs.NewWindow != nil {
				gotNewWindow = *lastArgs.NewWindow
			}
			if gotNewWindow != c.expectedNewWindow {
				t.Errorf("NewWindow parameter expected %v, but got %v", c.expectedNewWindow, gotNewWindow)
			}
			gotBackground := false
			if lastArgs.Background != nil {
				gotBackground = *lastArgs.Background
			}
			if gotBackground != c.expectedBackground {
				t.Errorf("Background parameter expected %v, but got %v", c.expectedBackground, gotBackground)
			}
		})
	}
}
