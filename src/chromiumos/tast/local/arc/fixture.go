// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "arcBooted",
		Desc: "ARC is booted",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedWithGpuWatchDog is a fixture similar to arcBooted. The only difference from arcBooted is that this fixture checks if there are any GPU-related problems (hangs + crashes) observed during a test.
	testing.AddFixture(&testing.Fixture{
		Name:   "arcBootedWithGpuWatchDog",
		Desc:   "ARC is booted with checking for GPU-related problems (hangs + crashes)",
		Parent: "gpuWatchDog",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedWithDisableSyncFlags is a fixture similar to arcBooted. The only difference from arcBooted is that ARC content sync is disabled to avoid noise during power/performance measurements.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithDisableSyncFlags",
		Desc: "ARC is booted with disabling sync flags",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.ExtraArgs(DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedInTabletMode is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched in tablet mode in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedInTabletMode",
		Desc: "ARC is booted in tablet mode",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedWithVideoLogging is a fixture similar to arcBooted, but with additional Chrome video logging enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithVideoLogging",
		Desc: "ARC is booted with additional Chrome video logging",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.ExtraArgs(
				"--vmodule=" + strings.Join([]string{
					"*/media/gpu/chromeos/*=2",
					"*/media/gpu/vaapi/*=2",
					"*/media/gpu/v4l2/*=2",
					"*/components/arc/video_accelerator/*=2"}, ","))}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})

	// arcBootedWithWebAppSharing is a fixture similar to arcBooted. The only difference is that the "ArcEnableWebAppShare" Chrome feature is enabled.
	// TODO(crbug.com/1226730): Remove and reuse arcBooted once ArcEnableWebAppShare is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithWebAppSharing",
		Desc: "ARC is booted with Web App sharing enabled",
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=ArcEnableWebAppShare")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: resetTimeout,
		TearDownTimeout: resetTimeout,
	})
}

type bootedFixture struct {
	cr   *chrome.Chrome
	arc  *ARC
	init *Snapshot

	playStoreOptin bool // Opt into PlayStore.

	fOpt chrome.OptionsCallback // Function to return chrome options.
}

// NewArcBootedFixture returns a FixtureImpl with a OptionsCallback function provided.
// ARCEnabled() will always be added to the Chrome options returned by OptionsCallback.
func NewArcBootedFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := fOpts(ctx, s)
			if err != nil {
				return nil, err
			}
			return append(opts, chrome.ARCEnabled(), chrome.ExtraArgs("--disable-features=ArcResizeLock")), nil
		},
	}
}

// NewArcBootedWithPlayStoreFixture returns a FixtureImpl with a OptionsCallback function
// provided.
// ARCSupported() will always be added to the Chrome options returned by OptionsCallback.
func NewArcBootedWithPlayStoreFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		playStoreOptin: true,
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := fOpts(ctx, s)
			if err != nil {
				return nil, err
			}
			return append(opts, chrome.ARCSupported()), nil
		},
	}
}

func (f *bootedFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	opts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain fixture options: ", err)
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		if !success {
			cr.Close(ctx)
		}
	}()

	if f.playStoreOptin {
		s.Log("Performing Play Store Optin")
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}
		st, err := GetState(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get ARC state: ", err)
		}
		if st.Provisioned {
			s.Log("ARC is already provisioned. Skipping the Play Store setup")
		} else {
			// Opt into Play Store and close the Play Store window.
			if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
				s.Fatal("Failed to opt into Play Store: ", err)
			}
		}
	}

	arc, err := New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if !success {
			arc.Close(ctx)
		}
	}()

	init, err := NewSnapshot(ctx, arc)
	if err != nil {
		s.Fatal("Failed to take ARC state snapshot: ", err)
	}

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this bootedFixture is active.
	Lock()
	chrome.Lock()

	f.cr = cr
	f.arc = arc
	f.init = init
	success = true
	return &PreData{
		Chrome: cr,
		ARC:    arc,
	}
}

func (f *bootedFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.arc.Close(ctx); err != nil {
		s.Log("Failed to close ARC: ", err)
	}
	f.arc = nil

	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome: ", err)
	}
	f.cr = nil
}

func (f *bootedFixture) Reset(ctx context.Context) error {
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}
	return f.init.Restore(ctx, f.arc)
}

func (f *bootedFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.

	if err := f.arc.resetOutDir(ctx, s.OutDir()); err != nil {
		s.Error("Failed to to reset outDir field of ARC object: ", err)
	}
}

func (f *bootedFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.

	if err := f.arc.saveLogFiles(ctx); err != nil {
		s.Error("Failed to to save ARC-related log files: ", err)
	}

	if s.HasError() {
		faillogDir := filepath.Join(s.OutDir(), "faillog")
		if err := os.MkdirAll(faillogDir, 0755); err != nil {
			s.Error("Failed to make faillog/ directory: ", err)
			return
		}
		if err := saveProcessList(ctx, f.arc, faillogDir); err != nil {
			s.Error("Failed to save the process list in ARCVM: ", err)
		}
	}
}

func saveProcessList(ctx context.Context, a *ARC, outDir string) error {
	path := filepath.Join(outDir, "ps-arcvm.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := a.Command(ctx, "ps", "-AfZ")
	cmd.Stdout = file
	return cmd.Run()
}
