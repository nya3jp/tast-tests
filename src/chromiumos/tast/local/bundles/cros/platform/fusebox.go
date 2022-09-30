// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fusebox,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Mount fusebox daemon and verify it responds to requests",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"nigeltao@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
	})
}

func Fusebox(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Logging into Chrome should launch Fusebox (via cros-disks).
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}

	// Poll until the "fuse_status" file shows up. The "fuse_status" and "ok\n"
	// magic strings are defined in "platform2/fusebox/built_in.cc".
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const fuseStatusFilename = "/media/fuse/fusebox/built_in/fuse_status"
		if got, err := os.ReadFile(fuseStatusFilename); err != nil {
			return err
		} else if want := "ok\n"; string(got) != want {
			return errors.Errorf("got %q, want %q", got, want)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("ReadFile(fuse_status) failed: ", err)
	}

	// Make a temporary directory.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	tdd := tempDirData{}
	if err := tconn.Call(ctx, &tdd, `tast.promisify(chrome.autotestPrivate.makeFuseboxTempDir)`); err != nil {
		s.Fatal("makeFuseboxTempDir failed: ", err)
	} else if tdd.FuseboxFilePath == "" {
		s.Fatal("FuseboxFilePath is empty")
	}
	defer tconn.Call(ctx, nil, `chrome.autotestPrivate.removeFuseboxTempDir`, tdd.FuseboxFilePath)

	// That temporary directory has two names: a fusebox one at
	// "/media/fuse/fusebox/tmp.foo" and an underlying one at "/tmp/.foo".
	// Creating "hello.txt" in the second should be visible in the first.
	// TODO(crbug.com/1360740): and vice versa.
	const world = "world\n"
	helloFuseboxFilename := filepath.Join(tdd.FuseboxFilePath, "hello.txt")
	helloUnderlyingFilename := filepath.Join(tdd.UnderlyingFilePath, "hello.txt")
	if err := os.WriteFile(helloUnderlyingFilename, []byte(world), 0777); err != nil {
		s.Fatal("WriteFile(hello.txt) failed: ", err)
	} else if got, err := os.ReadFile(helloFuseboxFilename); err != nil {
		s.Fatal("ReadFile(hello.txt) failed: ", err)
	} else if string(got) != world {
		s.Fatalf("ReadFile(hello.txt): got %q, want %q", got, world)
	}
}

type tempDirData struct {
	FuseboxFilePath    string `json:"fuseboxFilePath"`
	UnderlyingFilePath string `json:"underlyingFilePath"`
}
