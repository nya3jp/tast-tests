// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"io/ioutil"
	"math"
	"os"
	"testing"
)

func TestSimpleSuccess(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestSimpleSuccess")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "pid:4:1234"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if !found {
		t.Error("IsBreakpadDmpFileForPID returned false incorrectly")
	}
}

func TestWrongPID(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestWrongPID")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "pid:3:555"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if found {
		t.Error("IsBreakpadDmpFileForPID returned true incorrectly")
	}
}

func TestMultipleKeySuccess(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestMultipleKeySuccess")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browseruser:8:chromeospid:4:1234"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if !found {
		t.Error("IsBreakpadDmpFileForPID returned false incorrectly")
	}
}

func TestMultipleKeySuccess2(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestMultipleKeySuccess2")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browserpid:4:1234user:8:chromeos"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if !found {
		t.Error("IsBreakpadDmpFileForPID returned false incorrectly")
	}
}

func TestNoPIDKey(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestNoPIDKey")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browseruser:8:chromeos"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if found {
		t.Error("IsBreakpadDmpFileForPID returned true incorrectly")
	}
}

func TestMalformedIncompleteKey(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestMalformedIncompleteKey")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browseruser:8:chromeospid"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if _, err := IsBreakpadDmpFileForPID(fileName, 1234); err == nil {
		t.Errorf("IsBreakpadDmpFileForPID did not get error")
	}
}

func TestMalformedIncompleteLength(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestMalformedIncompleteLength")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browseruser:8:chromeospid:4"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if _, err = IsBreakpadDmpFileForPID(fileName, 1234); err == nil {
		t.Errorf("IsBreakpadDmpFileForPID did not get error")
	}
}

func TestMalformedIncompleteValue(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestMalformedIncompleteValue")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	const contents = "prod:15:Chrome_ChromeOSptype:7:browseruser:8:ch"
	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if _, err := IsBreakpadDmpFileForPID(fileName, 1234); err == nil {
		t.Errorf("IsBreakpadDmpFileForPID did not get error")
	}
}

func TestNonUTF8(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestNonUTF8")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(dir)

	contents := []byte("prod:15:Chrome_ChromeOSptype:7:browser")
	// Real .dmp files have a long section called the upload file minidump which
	// is not valid UTF8. Make sure we never cast the bytes of the file to a type
	// that expects valid UTF8.
	contents = append(contents, []byte(`upload_file_minidump"; filename="dump":100000:`)...)
	var b byte
	b = 0
	for i := 0; i < 100000; i++ {
		contents = append(contents, b)
		if b == math.MaxUint8 {
			b = 0
		} else {
			b++
		}
	}
	contents = append(contents, []byte("pid:4:1234done:3:yes")...)

	fileName := dir + "/test.dmp"

	if err = ioutil.WriteFile(fileName, []byte(contents), 0755); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if found, err := IsBreakpadDmpFileForPID(fileName, 1234); err != nil {
		t.Errorf("IsBreakpadDmpFileForPID got error: %v", err)
	} else if !found {
		t.Error("IsBreakpadDmpFileForPID returned false incorrectly")
	}
}

func TestIsFrameInStack(t *testing.T) {
	const basename = "platform.UserCrash.crasher"
	const recbomb = "recbomb"
	const bombSource = "platform.UserCrash.crasher.bomb.cc"
	const frame = 15
	const line = 12
	stack := []byte(`
    Found by: call frame info
15  platform.UserCrash.crasher!recbomb(int) [platform.UserCrash.crasher.bomb.cc : 12 + 0x5]
	rbx = 0x0000000000000000   rbp = 0x00007fffeb808c80`)
	for _, i := range []struct {
		frameIndex int
		module     string
		function   string
		file       string
		line       int
		expect     bool
	}{
		{frame, basename, recbomb, bombSource, line, true},
		{frame, basename, recbomb, bombSource, 333, false},
		{frame, basename, recbomb, "wrong.cc", line, false},
		{frame, basename, "wrong_function", bombSource, line, false},
		{frame, "wrong.BaseName", recbomb, bombSource, line, false},
		{99, basename, recbomb, bombSource, line, false},
	} {
		if found, _ := isFrameInStack(i.frameIndex, i.module, i.function, i.file, i.line, stack); found != i.expect {
			t.Errorf("failed: %v", i)
		}
	}
}
