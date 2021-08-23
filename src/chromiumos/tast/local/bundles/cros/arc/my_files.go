// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// myFilesUUID is the UUID of the MyFiles volume inside ARC. It is defined in
// Chromium's components/arc/volume_mounter/arc_volume_mounter_bridge.cc.
const myFilesUUID = "0000000000000000000000000000CAFEF00D2019"

func init() {
	testing.AddTest(&testing.Test{
		Func: MyFiles,
		Desc: "Checks whether the MyFiles directory is properly shared from Chrome OS to ARC",
		Contacts: []string{
			"youkichihosoi@chromium.org", "arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

func MyFiles(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	if err := waitForARCMyFilesMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.NormalizedUser(), err)
	}
	myFilesPath := cryptohomeUserPath + "/MyFiles"

	testARCToCros(ctx, s, a, myFilesPath)

	testCrosToARC(ctx, s, a, cr, d, myFilesPath)
}

// waitForARCMyFilesMount waits for the MyFiles volume to be mounted in ARC
// using the sm command. Just checking mountinfo here is not sufficient since
// it takes some time for the FUSE layer in Android R to be ready after
// /storage/<UUID> has become a mountpoint.
func waitForARCMyFilesMount(ctx context.Context, a *arc.ARC) error {
	// Regular expression that matches the output line for the mounted
	// MyFiles volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + myFilesUUID + `$`)

	testing.ContextLog(ctx, "Waiting for MyFiles to be mounted in ARC")

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "sm command failed")
		}
		lines := bytes.Split(out, []byte("\n"))
		for _, line := range lines {
			if re.Find(bytes.TrimSpace(line)) != nil {
				return nil
			}
		}
		return errors.New("MyFiles is not yet mounted in ARC")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// testARCToCros checks whether a file put in the Android MyFiles directory
// appears in the Chrome OS MyFiles directory.
func testARCToCros(ctx context.Context, s *testing.State, a *arc.ARC, myFilesPath string) {
	const (
		filename    = "capybara.jpg"
		androidPath = "/storage/" + myFilesUUID + "/" + filename
	)
	crosPath := myFilesPath + "/" + filename

	testing.ContextLog(ctx, "Testing Android -> CrOS")

	if err := testPushToARCAndReadFromCros(ctx, a, s.DataPath(filename), androidPath, crosPath); err != nil {
		s.Fatal("Android -> CrOS failed: ", err)
	}
}

// testPushToARCAndReadFromCros pushes the content of sourcePath (in Chrome OS)
// to androidPath (in Android) using adb, and then checks whether the file can
// be accessed under crosPath (in Chrome OS).
func testPushToARCAndReadFromCros(ctx context.Context, a *arc.ARC, sourcePath, androidPath, crosPath string) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	expected, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Chrome OS", sourcePath)
	}

	if err := a.WriteFile(ctx, androidPath, expected); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Android", androidPath)
	}
	defer func(ctx context.Context) {
		if err := a.RemoveAll(ctx, androidPath); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed remove %s in Android", androidPath)
			} else {
				testing.ContextLogf(ctx, "Failed to remove %s in Android: %v", androidPath, err)
			}
		}
	}(cleanupCtx)

	actual, err := ioutil.ReadFile(crosPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Chrome OS", crosPath)
	}
	if !bytes.Equal(actual, expected) {
		return errors.Errorf("content mismatch between %s in Android and %s in Chrome OS", androidPath, crosPath)
	}

	return nil
}

// testCrosToARC checks whether a file put in the Chrome OS MyFiles directory
// can be read by Android apps.
func testCrosToARC(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, d *ui.Device, myFilesPath string) {
	config := storage.TestConfig{DirPath: myFilesPath, DirName: "My files", DirTitle: "Files - My files",
		CreateTestFile: true, FileName: "storage.txt"}
	testFileURI := "content://org.chromium.arc.volumeprovider/" + myFilesUUID + "/" + config.FileName

	testing.ContextLog(ctx, "Testing CrOS -> Android")

	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: testFileURI},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expectations)
}
