// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RobloxUncompressOBBPerf,
		Desc: "Measures time it takes to uncompress Roblox OBB file",
		Contacts: []string{
			"ricardoq@chromium.org",
			"arc-performance+tast@google.com",
			"arc-gaming+tast@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"com.roblox.client-886.zip"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func RobloxUncompressOBBPerf(ctx context.Context, s *testing.State) {
	// Roblox itself measures the time it takes to uncompress the OBB file
	// and the result it logged in logcat.
	// This test just grabs that value from logcat and reports it.

	// Roblox has the OBB file built-in. This means that this test does not
	// download the OBB, or for the matter any file, from the internet.
	// However, Roblox requires internet to run. If internet is down, this test
	// will fail.

	// Roblox might trigger an ANR while uncompressing the OBB file (see: http://b/169182810).
	// In case the ANR dialog appears, it will be closed automatically at cleanup time,
	// when the Roblox APK gets uninstalled.

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Unzip "split" APK and install all the "splits" at the same time.
	tempDir, err := ioutil.TempDir("", "roblox-split-apk-")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(tempDir)

	const zipName = "com.roblox.client-886.zip"
	if err := testexec.CommandContext(ctx, "unzip", s.DataPath(zipName), "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to unzip %v from %v: %v", zipName, tempDir, err)
	}

	// These files belong to the .zip file.
	apks := []string{
		path.Join(tempDir, "com.roblox.client_886_base_split.apk"),
		path.Join(tempDir, "com.roblox.client_886_001.apk"),
		path.Join(tempDir, "com.roblox.client_886_002.apk"),
	}

	s.Log("Installing split APK")
	if err := a.InstallMultiple(ctx, apks); err != nil {
		s.Fatal("Failed installing split APK: ", err)
	}

	// To make test as stable as possible, wait until the CPU is "idle".
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU is idle: ", err)
	}

	const (
		pkgName = "com.roblox.client"
		actName = ".startup.ActivitySplash"
	)
	act, err := arc.NewActivity(a, pkgName, actName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Logf("Starting activity: %s/%s", pkgName, actName)
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	// Set timeout for the logcat command below.
	const logcatTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, logcatTimeout)
	defer cancel()

	// Reduce the scope of logcat entries. Searching for:
	// 2304  2304 D rbx.xapkmanager: [f.e()-193]: unpackAssets_internal: unzipContainer took (ms) 2266
	cmd := a.Command(ctx, "logcat", "-s", "rbx.xapkmanager:*", "-e", "unzipContainer")

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to obtain a pipe for ", cmd)
	}

	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start ", cmd)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	logcatRegexp := regexp.MustCompile(`unzipContainer took \(ms\) (\d+)`)

	s.Log(ctx, "Scanning logcat output")
	scanner := bufio.NewScanner(pipe)
	obbTimeMS := -1
	for scanner.Scan() {
		l := scanner.Text()
		m := logcatRegexp.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		obbTimeMS, err = strconv.Atoi(m[1])
		if err != nil {
			s.Log("OBB unzip time: ", obbTimeMS)
			s.Fatal("Failed to extract time from: ", l)
		}
		break
	}
	// Happens if scanner timesout.
	if obbTimeMS == -1 {
		s.Fatal("Failed to get OBB unzip time")
	}

	obbTimeMetric := perf.Metric{Name: "obb_unzip_time", Unit: "ms", Direction: perf.SmallerIsBetter}
	p := perf.NewValues()
	p.Set(obbTimeMetric, float64(obbTimeMS))
	if err = p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
