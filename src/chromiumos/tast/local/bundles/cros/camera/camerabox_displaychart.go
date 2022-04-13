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

	"chromiumos/tast/common/camera/chart"
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
	const image = "/usr/share/chromeos-assets/wallpaper/child_small.jpg"

	s.Log(ctx, "display chart testing start")

	logFile := filepath.Join(s.OutDir(), "camerabox_displaychart.log")

	s.Log("Run script: ", "python", chart.DisplayScript, image)
	s.Log("Log file store in :", logFile)

	cmd := testexec.CommandContext(ctx, "python", chart.DisplayScript, image)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Fatal("Failed to obtain a pipe for: ", err)
	}

	if err = cmd.Start(); err != nil {
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
		if err != nil {
			s.Fatal("Write file fail : ", err)
		}
		if strings.Contains(scanner.Text(), chart.ChartReadyMsg) {
			s.Log("Find out key word : ", scanner.Text())
			found = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		s.Fatal("stderr scanner error: ", err)
	}
	if found == false {
		s.Fatal("Can not find  ", chart.ChartReadyMsg)
	}

	s.Log(ctx, "display chart testing done")
}
