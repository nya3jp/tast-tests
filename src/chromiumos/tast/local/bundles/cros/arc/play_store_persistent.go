// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayStorePersistent,
		Desc:         "Makes sure that Play Store remains open after it is fully initialized",
		Contacts:     []string{"khmel@chromium.org", "jhorwich@chromium.org", "arc-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// 1 min for ARC is provisioned, 4 minutes max waiting for daily hygiene, and
		// 1 min max waiting for CPU is idle. Normally test takes ~2.5-3.5 minutes to complete.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_all"},
			Val:               []string{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               []string{"--enable-arcvm"},
		}},
	})
}

// getPlayStorePid gets the PID of Play Store application.
func getPlayStorePid(ctx context.Context, a *arc.ARC) (uint, error) {
	out, err := a.Command(ctx, "pidof", "com.android.vending").Output()
	if err != nil {
		return 0, err
	}

	m := regexp.MustCompile(`(\d+)\n`).FindAllStringSubmatch(string(out), -1)
	if m == nil || len(m) != 1 {
		return 0, errors.New("could not find Play Store app")
	}

	pid, err := strconv.ParseUint(m[0][1], 10, 32)
	if err != nil {
		return 0, err
	}

	return uint(pid), nil
}

// readFinskyPrefs reads content of Finsky shared prefs file.
func readFinskyPrefs(ctx context.Context) ([]byte, error) {
	const finskyPrefs = "/data/data/com.android.vending/shared_prefs/finsky.xml"

	// adb pull would fail due to permissions limitation. Use bootstrapped cat to copy it.
	// TODO(b/148832630): get rid of BootstrapCommand.
	cmd := arc.BootstrapCommand(ctx, "/bin/cat", finskyPrefs)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// waitForDailyHygieneDone waits for Play Store daily hygiene is done. dailyhygiene-last-version
// in shared Finsky pref is set in case this flow is finished. Usually this happens in 2 minutes.
// At this moment, Play Store self-update might be executing. This also handles the case when
// daily hygiene fails internally. This is not ARC fault and we detect this as a signal that
// daily hygiene ends. Next potentially successful attempt should happen in 20 min which is
// problematic to wait in test.
func waitForDailyHygieneDone(ctx context.Context, a *arc.ARC) (bool, error) {
	reOk := regexp.MustCompile(`<int name="dailyhygiene-last-version" value="\d+"`)
	reFail := regexp.MustCompile(`<int name="dailyhygiene-failed" value="1" />`)
	var ok bool
	return ok, testing.Poll(ctx, func(ctx context.Context) error {
		out, err := readFinskyPrefs(ctx)
		if err != nil {
			// It is OK if it does not exist yet
			return err
		}

		if reOk.Find(out) != nil {
			ok = true
			return nil
		}

		if reFail.Find(out) != nil {
			ok = false
			return nil
		}

		return errors.New("dailyhygiene is not yet complete")
	}, &testing.PollOptions{Timeout: 4 * time.Minute, Interval: 5 * time.Second})
}

func PlayStorePersistent(ctx context.Context, s *testing.State) {
	extraArgs := s.Param().([]string)
	args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"}
	args = append(args, extraArgs...)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	pidBefore, err := getPlayStorePid(ctx, a)
	if err != nil {
		s.Fatal("Failed to get initial PlayStore PID: ", err)
	}

	s.Log("Wating for daily hygiene done")
	ok, err := waitForDailyHygieneDone(ctx, a)
	if err != nil {
		if out, rerr := readFinskyPrefs(ctx); rerr != nil {
			s.Error("Failed to read Finsky prefs: ", rerr)
		} else if rerr := ioutil.WriteFile(filepath.Join(s.OutDir(), "finsky.xml"), out, 0644); rerr != nil {
			s.Error("Failed to write Finsky prefs: ", rerr)
		} else {
			s.Log("Finsky prefs is saved to finsky.xml")
		}
		s.Log("Failed to wait daily hygiene done")
	}

	if ok {
		s.Log("Daily hygiene finished successfully")
	} else {
		s.Log("Daily hygiene failed but continue")
	}

	// Daily hygiene may start the self-update flow and now system is busy. This waiting just waits
	// everything is stabilized. That means new Play Store is installed if self-update flow was
	// started.
	s.Log("Wating for CPU idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait CPU is idle: ", err)
	}

	pidAfter, err := getPlayStorePid(ctx, a)
	if err != nil {
		s.Fatal("Failed to get PlayStore PID: ", err)
	}

	if pidAfter != pidBefore {
		s.Fatal("Play Store was restarted")
	}
}
