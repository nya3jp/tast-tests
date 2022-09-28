// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

// PreTestTimeout is the timeout duration to reset output directory before each test.
const PreTestTimeout = 15 * time.Second

// PostTestTimeout is the timeout duration to save logs after each test.
// It's intentionally set longer than ResetTimeout because dumping 'dumpsys' takes around 20 seconds.
const PostTestTimeout = ResetTimeout + 20*time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "arcBooted",
		Desc: "ARC is booted",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithoutUIAutomator is a fixture similar to arcBooted. The only difference from arcBooted is that UI Automator is not enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithoutUIAutomator",
		Desc: "ARC is booted without UI Automator",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedWithoutUIAutomatorFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithDarkMode is a fixture similar to arcBooted. The only difference from arcBooted is that Dark Light Mode is enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithDarkMode",
		Desc: "ARC is booted with Dark Mode",
		Contacts: []string{
			"ttefera@google.com",
			"arc-app-dev@google.com",
		},
		Impl: NewArcBootedWithDarkModeFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU(), chrome.EnableFeatures("DarkLightMode")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithDisableSyncFlags is a fixture similar to arcBooted. The only difference from arcBooted is that ARC content sync is disabled to avoid noise during power/performance measurements.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithDisableSyncFlags",
		Desc: "ARC is booted with disabling sync flags",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.ExtraArgs(DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedRestricted is a fixture similar to arcBootedWithDisableSyncFlags. The only difference
	// from arcBootedWithDisableSyncFlags is that CGroups is used to limit the CPU time of ARC, and
	// that Chrome will not check for firmware updates.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedRestricted",
		Desc: "ARC is booted in idle state",
		Contacts: []string{
			"alanding@chromium.org",
			"arc-performance@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.ExtraArgs(DisableSyncFlags()...),
				chrome.ExtraArgs("--disable-features=FirmwareUpdaterApp"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithPlayStore is a fixture similar to arcBooted along with GAIA login and Play Store Optin.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithPlayStore",
		Desc: "ARC is booted with disabling sync flags",
		Vars: []string{"ui.gaiaPoolDefault"},
		Contacts: []string{
			"rnanjappan@chromium.org",
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(DisableSyncFlags()...),
				chrome.UnRestrictARCCPU(),
				chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + BootTimeout + 2*time.Minute,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedInTabletMode is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched in tablet mode in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedInTabletMode",
		Desc: "ARC is booted in tablet mode",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedInClamshellMode is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched in clamshell mode in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedInClamshellMode",
		Desc: "ARC is booted in clamshell mode",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.ExtraArgs("--force-tablet-mode=clamshell"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithVideoLogging is a fixture similar to arcBooted, but with additional Chrome video logging enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithVideoLogging",
		Desc: "ARC is booted with additional Chrome video logging",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU(), chrome.ExtraArgs(
				"--vmodule=" + strings.Join([]string{
					"*/media/gpu/chromeos/*=2",
					"*/media/gpu/vaapi/*=2",
					"*/media/gpu/v4l2/*=2",
					"*/components/arc/video_accelerator/*=2"}, ","))}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithOutOfProcessVideoDecoding is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched with out-of-process
	// video decoding in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithOutOfProcessVideoDecoding",
		Desc: "ARC is booted with out-of-process video decoding",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.ExtraArgs("--enable-features=OutOfProcessVideoDecoding"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding is a fixture similar to arcBootedWithVideoLogging, but Chrome is launched with out-of-process video
	// decoding.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithVideoLoggingAndOutOfProcessVideoDecoding",
		Desc: "ARC is booted with out-of-process video decoding and additional Chrome video logging",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU(), chrome.ExtraArgs(
				"--enable-features=OutOfProcessVideoDecoding",
				"--vmodule="+strings.Join([]string{
					"*/media/gpu/chromeos/*=2",
					"*/media/gpu/vaapi/*=2",
					"*/media/gpu/v4l2/*=2",
					"*/components/arc/video_accelerator/*=2"}, ","))}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithVideoLoggingVD is a fixture similar to arcBootedWithVideoLogging, but with additional Chrome
	// video logging enabled and the mojo::VideoDecoder stack enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithVideoLoggingVD",
		Desc: "ARC is booted with VD and additional Chrome video logging",
		Contacts: []string{
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedWithConfigFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(), chrome.UnRestrictARCCPU(), chrome.ExtraArgs(
				"--vmodule=" + strings.Join([]string{
					"*/media/gpu/chromeos/*=2",
					"*/media/gpu/vaapi/*=2",
					"*/media/gpu/v4l2/*=2",
					"*/components/arc/video_accelerator/*=2"}, ","))}, nil
		}, "!--video-decoder\n--video-decoder=libvda-vd\n"),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// lacrosWithArcBooted is a fixture that combines the functionality of arcBooted and lacros.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWithArcBooted",
		Desc:     "Lacros Chrome from a pre-built image with ARC booted",
		Contacts: []string{"amusbach@chromium.org", "xiyuan@chromium.org"},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chrome.ARCEnabled(), chrome.UnRestrictARCCPU())).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedInClamshellMode is a fixture similar to arcBooted. The only difference from arcBooted is that Chrome is launched in clamshell mode with Touch Mode Mouse compat features enabled in this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithTouchModeMouse",
		Desc: "ARC is booted in clamshell mode with Touch Mode Mouse compat features enabled",
		Contacts: []string{
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.EnableFeatures("ArcRightClickLongPress"),
				chrome.ExtraArgs("--force-tablet-mode=clamshell"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// TODO(b/216709995): Remove this after the feature is launched.
	// arcBootedWithNotificationRefresh is a fixture similar to arcBooted but with notification-refresh flag enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithNotificationRefresh",
		Desc: "ARC is booted with the notification-refresh flag enabled",
		Contacts: []string{
			"toshikikikuchi@chromium.org",
			"niwa@chromium.org",
			"arcvm-eng-team@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.EnableFeatures("NotificationsRefresh"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// arcBootedWithInputOverlay is a fixture similar to arcBooted but with the input overlay flag enabled.
	testing.AddFixture(&testing.Fixture{
		Name: "arcBootedWithInputOverlay",
		Desc: "ARC is booted with the input overlay flag enabled",
		Contacts: []string{
			"pjlee@google.com",
			"cuicuiruan@google.com",
			"arc-app-dev@google.com",
		},
		Impl: NewArcBootedFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ARCEnabled(),
				chrome.UnRestrictARCCPU(),
				chrome.ExtraArgs("--enable-features=ArcInputOverlay"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + BootTimeout + ui.StartTimeout,
		ResetTimeout:    ResetTimeout,
		PostTestTimeout: PostTestTimeout,
		TearDownTimeout: ResetTimeout,
	})
}

type bootedFixture struct {
	cr   *chrome.Chrome
	arc  *ARC
	d    *ui.Device
	init *Snapshot

	playStoreOptin    bool   // Opt into PlayStore.
	enableUIAutomator bool   // Enable UI Automator
	arcvmConfig       string // Append config to arcvm_dev.conf

	fOpt chrome.OptionsCallback // Function to return chrome options.
}

// NewArcBootedFixture returns a FixtureImpl with a OptionsCallback function provided.
// ARCEnabled() will always be added to the Chrome options returned by OptionsCallback.
func NewArcBootedFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return NewArcBootedWithConfigFixture(fOpts, "")
}

// NewArcBootedWithConfigFixture returns a FixtureImpl with a OptionsCallback function provided and
// the specified config appended to arcvm_dev.conf. ARCEnabled() will always be added to the Chrome
// options returned by OptionsCallback.
func NewArcBootedWithConfigFixture(fOpts chrome.OptionsCallback, arcvmConfig string) testing.FixtureImpl {
	return &bootedFixture{
		enableUIAutomator: true,
		arcvmConfig:       arcvmConfig,
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := fOpts(ctx, s)
			if err != nil {
				return nil, err
			}
			return append(opts, chrome.ARCEnabled(), chrome.ExtraArgs("--disable-features=ArcResizeLock")), nil
		},
	}
}

// NewMtbfArcBootedFixture returns a FixtureImpl with a OptionsCallback function provided for MTBF ARC++ tests.
func NewMtbfArcBootedFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		enableUIAutomator: false,
		playStoreOptin:    true,
		fOpt:              fOpts,
	}
}

// NewArcBootedWithoutUIAutomatorFixture is same as NewArcBootedFixture but does not install UIAutomator by default.
func NewArcBootedWithoutUIAutomatorFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		enableUIAutomator: false,
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := fOpts(ctx, s)
			if err != nil {
				return nil, err
			}
			return append(opts, chrome.ARCEnabled(), chrome.ExtraArgs("--disable-features=ArcResizeLock")), nil
		},
	}
}

// NewArcBootedWithDarkModeFixture is same as NewArcBootedFixture but enables Dark Light Mode.
func NewArcBootedWithDarkModeFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		enableUIAutomator: false,
		fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := fOpts(ctx, s)
			if err != nil {
				return nil, err
			}
			return append(opts, chrome.ARCEnabled(), chrome.ExtraArgs("--disable-features=ArcResizeLock"), chrome.EnableFeatures("DarkLightMode")), nil
		},
	}
}

// NewArcBootedWithPlayStoreFixture returns a FixtureImpl with a OptionsCallback function
// provided.
// ARCSupported() will always be added to the Chrome options returned by OptionsCallback.
func NewArcBootedWithPlayStoreFixture(fOpts chrome.OptionsCallback) testing.FixtureImpl {
	return &bootedFixture{
		playStoreOptin:    true,
		enableUIAutomator: true,
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

	// Append additional config to the ARCVM config file, needs to be done before launching Chrome.
	if f.arcvmConfig != "" {
		if err := AppendToArcvmDevConf(ctx, f.arcvmConfig); err != nil {
			s.Fatal("Failed to write arcvm_dev.conf: ", err)
		}
	}
	defer func() {
		if !success && f.arcvmConfig != "" {
			if err := RestoreArcvmDevConf(ctx); err != nil {
				s.Fatal("Failed to restore arcvm_dev.conf: ", err)
			}
		}
	}()

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

	var d *ui.Device
	if f.enableUIAutomator {
		if d, err = arc.NewUIDevice(s.FixtContext()); err != nil {
			s.Fatal("Failed to initialize UI Automator: ", err)
		}
		defer func() {
			if !success {
				d.Close(ctx)
			}
		}()
	}

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
	f.d = d
	f.init = init
	success = true
	return &PreData{
		Chrome:   cr,
		ARC:      arc,
		UIDevice: d,
	}
}

func (f *bootedFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.arcvmConfig != "" {
		if err := RestoreArcvmDevConf(ctx); err != nil {
			s.Fatal("Failed to restore arcvm_dev.conf: ", err)
		}
	}

	if f.d != nil {
		if err := f.d.Close(ctx); err != nil {
			s.Log("Failed to close UI Automator: ", err)
		}
		f.d = nil
	}

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
	if f.d != nil && !f.d.Alive(ctx) {
		return errors.New("UI Automator is dead")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}
	return f.init.Restore(ctx, f.arc)
}

func (f *bootedFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.

	if err := f.arc.ResetOutDir(ctx, s.OutDir()); err != nil {
		s.Error("Failed to to reset outDir field of ARC object: ", err)
	}
}

func (f *bootedFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1136382): Support per-test logcat once we get pre/post-test
	// hooks in fixtures.

	if err := f.arc.SaveLogFiles(ctx); err != nil {
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
		if err := saveDumpsys(ctx, f.arc, faillogDir); err != nil {
			s.Error("Failed to save dumpsys output in ARCVM: ", err)
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

func saveDumpsys(ctx context.Context, a *ARC, outDir string) error {
	path := filepath.Join(outDir, "dumpsys.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := a.Command(ctx, "dumpsys")
	cmd.Stdout = file
	return cmd.Run()
}
