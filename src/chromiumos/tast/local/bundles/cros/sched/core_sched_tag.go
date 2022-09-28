// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sched contains scheduler-related ChromeOS tests
package sched

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/lacros/lacrosproc"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CoreSchedTag,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Ensures renderers scheduling cookies are assigned correctly",
		Contacts:     []string{"joelaf@google.com", "briannorris@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "arc", "coresched"},
		HardwareDeps: hwdep.D(hwdep.CPUSupportsSMT()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func getProcCookie(p *process.Process) (int64, error) {
	path := filepath.Join("/proc", fmt.Sprint(p.Pid), "sched")

	re, err := regexp.Compile(`core_cookie\s*:\s*(.+)`)
	if err != nil {
		return 0, errors.Wrap(err, "failed to compile cookie regex")
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	res := re.FindAllSubmatch(data, -1)
	if len(res) == 0 {
		return 0, errors.New("failed to find core_cookie in sched file")
	}

	f, err := strconv.ParseInt(string(res[0][1]), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert cookie to int")
	}
	return f, nil
}

func getThreadsFromProcess(p *process.Process) ([]*process.Process, error) {
	path := filepath.Join("/proc", fmt.Sprint(p.Pid), "task")
	var ret []*process.Process

	finfos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, finfo := range finfos {
		fname := finfo.Name()

		pid, err := strconv.ParseInt(fname, 10, 32)
		if err != nil {
			// if not numeric name, just skip
			continue
		}

		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}

		ret = append(ret, proc)
	}

	return ret, nil
}

// verifyTags verifies the tags of all renderer and ARC processes (TODO: add ARC).
// Make sure that the ones in containsPids are scanned (This is to ensure that
// chrome is among the processes scanned.)
func verifyTags(ctx context.Context, tconn *chrome.TestConn, browserType browser.Type) error {
	cookieMap := make(map[int64]bool)

	procs, err := chromeproc.GetRendererProcesses()
	if err != nil {
		return errors.Wrap(err, "failed to get renderer processes")
	}

	if browserType == browser.TypeLacros {
		lacrosProcs, err := lacrosproc.RendererProcesses(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros renderers")
		}
		procs = append(procs, lacrosProcs...)
	}

	for _, proc := range procs {
		parentCookie, _ := getProcCookie(proc)
		if parentCookie == 0 {
			return errors.Errorf("main thread of %d not tagged", proc.Pid)
		}

		if _, ok := cookieMap[parentCookie]; ok {
			return errors.New("duplicate renderer cookie or collision")
		}
		cookieMap[parentCookie] = true

		threads, err := getThreadsFromProcess(proc)
		if err != nil {
			return errors.Wrap(err, "failed to get threads in process")
		}

		for _, thread := range threads {
			cookie, err := getProcCookie(thread)
			if err != nil {
				return errors.Wrap(err, "failed to get cookie")
			}

			if cookie == 0 {
				return errors.New("renderer thread not tagged")
			}

			if cookie != parentCookie {
				return errors.New("main thread and renderer thread mismatch")
			}
		}
	}

	return nil
}

// CoreSchedTag : Function to test core scheduling cookies on ChromeOS
func CoreSchedTag(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	browserType := s.Param().(browser.Type)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	settingsConn, err := br.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer settingsConn.Close()

	if err := webutil.WaitForQuiescence(ctx, settingsConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	versionConn, err := br.NewConn(ctx, "chrome://version")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer versionConn.Close()

	if err := webutil.WaitForQuiescence(ctx, versionConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	if err := verifyTags(ctx, tconn, browserType); err != nil {
		s.Fatal("Failed to verify tags: ", err)
	}
}
