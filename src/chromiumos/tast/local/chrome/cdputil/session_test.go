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

type dummyConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan uint64
}

func (dc *dummyConn) Read(p []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (dc *dummyConn) Write(p []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (dc *dummyConn) Close() error {
	dc.cancel()
	close(dc.ch)
	return nil
}

func newDummyConn(ctx context.Context, addr string) (io.ReadWriteCloser, error) {
	dc := &dummyConn{}
	dc.ctx, dc.cancel = context.WithCancel(ctx)
	dc.ch = make(chan uint64)
	return dc, nil
}

type dummyCodec struct {
	ctx             context.Context
	ch              chan uint64
	lastRequestArgs interface{}
	nextResponse    interface{}
}

func (dc *dummyCodec) ReadResponse(resp *rpcc.Response) error {
	var id uint64
	select {
	case id = <-dc.ch:
		break
	case <-dc.ctx.Done():
		return dc.ctx.Err()
	}

	if dc.nextResponse != nil {
		var err error
		if resp.Result, err = json.Marshal(dc.nextResponse); err != nil {
			return err
		}
	} else {
		resp.Error = &rpcc.ResponseError{
			Message: "nextResponse does not exist",
		}
	}
	resp.ID = id
	return nil
}

func (dc *dummyCodec) WriteRequest(req *rpcc.Request) error {
	dc.lastRequestArgs = req.Args
	select {
	case dc.ch <- req.ID:
		return nil
	case <-dc.ctx.Done():
		return dc.ctx.Err()
	}
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
			dc := &dummyCodec{}
			conn, err := rpcc.Dial("dummy", rpcc.WithDialer(newDummyConn), rpcc.WithCodec(func(conn io.ReadWriter) rpcc.Codec {
				dconn := conn.(*dummyConn)
				dc.ctx = dconn.ctx
				dc.ch = dconn.ch
				return dc
			}))
			if err != nil {
				t.Fatal("Failed to dial: ", err)
			}
			sess := &Session{client: cdp.NewClient(conn)}
			dc.nextResponse = &target.CreateTargetReply{"dummy-id"}
			id, err := sess.CreateTarget(ctx, "about:blank", c.opts...)
			if err != nil {
				t.Fatal("Failed to call CreateTarget: ", err)
			}
			if id != "dummy-id" {
				t.Errorf("Expected dummy-id, got %s", id)
			}
			if dc.lastRequestArgs == nil {
				t.Fatal("The request doesn't arrive to rpcc")
			}
			lastArgs, ok := dc.lastRequestArgs.(*target.CreateTargetArgs)
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
