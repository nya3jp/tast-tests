// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartAndFinishCrashTest(t *testing.T) {
	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	dir := filepath.Join("/tmp", "TestStartAndFinishCrashTest", time.Now().String())
	if err := startCrashTestWithDirectory(dir); err != nil {
		t.Fatalf("startCrashTestWithDirectory(%s): %v", dir, err)
	}

	file := filepath.Join(dir, "crash-test-in-progress")
	if _, err := os.Stat(file); err != nil {
		t.Errorf("Cannot stat %s: %v", file, err)
	}

	if err := finishCrashTestWithDirectory(dir); err != nil {
		t.Errorf("finishCrashTestWithDirectory(%s): %v", dir, err)
	}

	_, err := os.Stat(file)
	if err == nil {
		t.Errorf("%s not deleted", file)
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error stating %s: %v", file, err)
	}
}
