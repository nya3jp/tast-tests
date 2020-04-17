// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package baserpc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/testutil"
)

// startTestPair starts a pair of remote file system server and client.
// It aborts the test when it encounters an error during setup. Callers are
// responsible for releasing the returned resources.
func startTestPair(t *testing.T) (*grpc.Server, *grpc.ClientConn) {
	t.Helper()

	s := grpc.NewServer()
	// Note: We omit releasing s on setup errors because this function is for
	// unit tests only and errors are rare.
	baserpc.RegisterFileSystemServer(s, &FileSystemService{nil})

	lis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal("Failed to listen: ", err)
	}
	go s.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		t.Fatal("Failed to dial: ", err)
	}
	return s, conn
}

func TestReadDir(t *testing.T) {
	srv, conn := startTestPair(t)
	defer srv.Stop()
	defer conn.Close()

	cl := dutfs.NewClient(conn)

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	fis, err := cl.ReadDir(context.Background(), dir)
	if err != nil {
		t.Error("ReadDir failed for empty directory: ", err)
	} else if len(fis) > 0 {
		t.Errorf("ReadDir returned %d entries for empty directory; want 0", len(fis))
	}

	if err := testutil.WriteFiles(dir, map[string]string{
		"foo": "12345678",
		"bar": "12345",
	}); err != nil {
		t.Fatal("Failed to write files: ", err)
	}

	fis, err = cl.ReadDir(context.Background(), dir)
	if err != nil {
		t.Error("ReadDir failed for non-empty directory: ", err)
	} else {
		var got []string
		for _, fi := range fis {
			got = append(got, fmt.Sprintf("%s %d", fi.Name(), fi.Size()))
		}
		want := []string{
			"bar 5",
			"foo 8",
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Error("ReadDir returned unexpected entries for non-empty directory (-got +want):\n", diff)
		}
	}

	_, err = cl.ReadDir(context.Background(), filepath.Join(dir, "no_such_dir"))
	if !os.IsNotExist(err) {
		t.Errorf("ReadDir: %v; want %v", err, os.ErrNotExist)
	}
}

func TestStat(t *testing.T) {
	srv, conn := startTestPair(t)
	defer srv.Stop()
	defer conn.Close()

	cl := dutfs.NewClient(conn)

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "foo")
	if err := ioutil.WriteFile(path, []byte("12345"), 0666); err != nil {
		t.Fatal("Failed to create file: ", err)
	}

	fi, err := cl.Stat(context.Background(), dir)
	if err != nil {
		t.Error("Stat failed for directory: ", err)
	} else {
		if !fi.IsDir() {
			t.Error("IsDir = false for directory")
		}
	}

	fi, err = cl.Stat(context.Background(), path)
	if err != nil {
		t.Error("Stat failed for file: ", err)
	} else {
		if fi.IsDir() {
			t.Error("IsDir = true for file")
		}
	}

	_, err = cl.Stat(context.Background(), filepath.Join(dir, "no_such_file"))
	if !os.IsNotExist(err) {
		t.Errorf("Stat: %v; want %v", err, os.ErrNotExist)
	}
}

func TestReadFile(t *testing.T) {
	srv, conn := startTestPair(t)
	defer srv.Stop()
	defer conn.Close()

	cl := dutfs.NewClient(conn)

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "foo")
	const content = "12345"
	if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
		t.Fatal("Failed to create file: ", err)
	}

	data, err := cl.ReadFile(context.Background(), path)
	if err != nil {
		t.Error("ReadFile failed: ", err)
	} else if s := string(data); s != content {
		t.Errorf("ReadFile returned %q; want %q", s, content)
	}

	_, err = cl.ReadFile(context.Background(), filepath.Join(dir, "no_such_file"))
	if !os.IsNotExist(err) {
		t.Errorf("ReadFile: %v; want %v", err, os.ErrNotExist)
	}
}
