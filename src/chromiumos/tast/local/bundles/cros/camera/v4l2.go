// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const hwTimestampsPath = "/sys/module/uvcvideo/parameters/hwtimestamps"

func init() {
	testing.AddTest(&testing.Test{
		Func: V4L2,
		Desc: "Verifies required V4L2 operations on USB camera devices",
		Contacts: []string{
			"kamesan@chromium.org",
			"shik@chromium.org",
			"henryhsu@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
	})
}

func V4L2(ctx context.Context, s *testing.State) {
	hasHWTimestamps, err := pathExist(hwTimestampsPath)
	if err != nil {
		s.Fatal("Failed to check hardware timestamps: ", err)
	}
	if hasHWTimestamps {
		origVal, err := setHWTimestamps("1")
		if err != nil {
			s.Fatal("Failed to set hardware timestamps: ", err)
		}
		defer setHWTimestamps(origVal)
	}

	testList, err := getTestList(ctx)
	if err != nil {
		s.Fatal("Failed to get test list: ", err)
	}

	usbCams, err := testutil.GetUSBCamerasFromV4L2Test(ctx)
	if err != nil {
		s.Fatal("Failed to get USB cameras: ", err)
	}
	if len(usbCams) == 0 {
		s.Fatal("Failed to find any valid device")
	}
	s.Log("USB cameras: ", usbCams)

	for _, devicePath := range usbCams {
		extraArgs := []string{
			"--device_path=" + devicePath,
			"--test_list=" + testList,
		}

		logFile := fmt.Sprintf("media_v4l2_test_%s.log", filepath.Base(devicePath))

		t := gtest.New("media_v4l2_test",
			gtest.Logfile(filepath.Join(s.OutDir(), logFile)),
			gtest.ExtraArgs(extraArgs...))

		if args, err := t.Args(); err == nil {
			s.Log("Running " + shutil.EscapeSlice(args))
		}

		report, err := t.Run(ctx)
		if err != nil {
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Error(name, " failed")
				}
			}
			s.Fatal("Failed to run media_v4l2_test: ", err)
		}
	}

}

func pathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func setHWTimestamps(newValue string) (oldValue string, err error) {
	b, err := ioutil.ReadFile(hwTimestampsPath)
	if err != nil {
		return "", err
	}
	oldValue = string(b)

	if err := ioutil.WriteFile(hwTimestampsPath, []byte(newValue), 0644); err != nil {
		return "", err
	}
	return oldValue, err
}

func getTestList(ctx context.Context) (string, error) {
	hasV3, err := pathExist("/usr/bin/cros_camera_service")
	if err != nil {
		return "", err
	}

	isV1, err := hal3.IsV1Legacy(ctx)
	if err != nil {
		return "", err
	}

	if hasV3 && !isV1 {
		return "halv3", nil
	}

	return "default", nil
}
