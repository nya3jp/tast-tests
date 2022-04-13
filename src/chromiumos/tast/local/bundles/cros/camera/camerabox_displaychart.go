// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CameraboxDisplaychart,
		Desc:     "Verifies whether display chart script working normally",
		Contacts: []string{"beckerh@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func CameraboxDisplaychart(ctx context.Context, s *testing.State) {
	const Image = "/usr/share/chromeos-assets/wallpaper/child_small.jpg"
	const DisplayScript = "/usr/local/autotest/bin/display_chart.py"
	const ChartIsReady = "Chart is ready"

	s.Log(ctx, "display chart testing start")

	logFile := filepath.Join(s.OutDir(), "camerabox_displaychart.log")

	//Run python command
	//The bash trick here is inevitable due to
	//the script will be run on another chart device instead of DUT without gRPC support.
	//So we cannot completely start python script in golang library
	//And since this test is kind of backing up for the remote test,
	//It also make sense to make them share more code as possible.

	s.Log("Run script: ", "python", DisplayScript, Image)
	s.Log("Log file store in :", logFile)

	cmd := testexec.CommandContext(ctx, "python", DisplayScript, Image)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Fatal("Failed to obtain a pipe for: ", err)
	}

	err = cmd.Start()
	if err != nil {
		s.Fatal("fail to start ", cmd)
	}

	defer func() {
		cmd.Kill()
		if err = cmd.Wait(); err != nil {
			s.Log("Kill python script sub rotinue status : ", err)
		}
	}()

	f, err := os.Create(logFile)
	if err != nil {
		s.Fatal("open file error : ", err, logFile)
	}
	defer f.Close()

	scanner := bufio.NewScanner(stderr)
	found := false
	for scanner.Scan() {
		_, err = f.WriteString(scanner.Text() + "\n")
		if strings.Contains(scanner.Text(), ChartIsReady) {
			s.Log("Find out key word : ", scanner.Text())
			found = true
			break
		}
	}
	if found == false {
		s.Fatal("Can not find  ", ChartIsReady)
	}

	testing.ContextLog(ctx, "display chart testing done")
}
