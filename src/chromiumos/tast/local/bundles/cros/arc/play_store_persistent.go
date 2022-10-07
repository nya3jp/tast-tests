// Copyright 2019 The ChromiumOS Authors
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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayStorePersistent,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Makes sure that Play Store remains open after it is fully initialized",
		Contacts:     []string{"khmel@chromium.org", "jhorwich@chromium.org", "arc-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc_android_data_cros_access", "chrome"},
		// 1 min for ARC is provisioned, 4 minutes max waiting for daily hygiene, and
		// 1 min max waiting for CPU is idle. Normally test takes ~2.5-3.5 minutes to complete.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Val:               browser.TypeLacros,
		}},
		VarDeps: []string{"ui.gaiaPoolDefault"},
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
func readFinskyPrefs(ctx context.Context, a *arc.ARC, user string) ([]byte, error) {
	const finskyPrefsPath = "/data/data/com.android.vending/shared_prefs/finsky.xml"

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(ctx, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	cleanupFunc, err := arc.MountVirtioBlkDataDiskImageReadOnlyIfUsed(ctx, a, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mount Android /data virtio-blk disk image on host")
	}
	defer cleanupFunc(cleanupCtx)

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	androidDataDir := filepath.Join(rootCryptDir, "android-data")

	return ioutil.ReadFile(filepath.Join(androidDataDir, finskyPrefsPath))
}

// waitForDailyHygieneDone waits for Play Store daily hygiene is done. dailyhygiene-last-version
// in shared Finsky pref is set in case this flow is finished. Usually this happens in 2 minutes.
// At this moment, Play Store self-update might be executing. This also handles the case when
// daily hygiene fails internally. This is not ARC fault and we detect this as a signal that
// daily hygiene ends. Next potentially successful attempt should happen in 20 min which is
// problematic to wait in test.
func waitForDailyHygieneDone(ctx context.Context, a *arc.ARC, user string) (bool, error) {
	reOk := regexp.MustCompile(`<int name="dailyhygiene-last-version" value="\d+"`)
	reFail := regexp.MustCompile(`<int name="dailyhygiene-failed" value="1" />`)
	var ok bool
	return ok, testing.Poll(ctx, func(ctx context.Context) error {
		// On ARCVM virtio-blk /data enabled devices, we mount and unmount the disk image on
		// every iteration of testing.Poll to ensure that the Android-side changes are
		// reflected on the host side.
		out, err := readFinskyPrefs(ctx, a, user)
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	opts := []chrome.Option{
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
	}

	bt := s.Param().(browser.Type)
	cr, err := browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := 2
	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	pidBefore, err := getPlayStorePid(ctx, a)
	if err != nil {
		s.Fatal("Failed to get initial PlayStore PID: ", err)
	}

	s.Log("Waiting for daily hygiene done")
	ok, err := waitForDailyHygieneDone(ctx, a, cr.NormalizedUser())
	if err != nil {
		if out, rerr := readFinskyPrefs(ctx, a, cr.NormalizedUser()); rerr != nil {
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
	s.Log("Waiting for CPU idle")
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
