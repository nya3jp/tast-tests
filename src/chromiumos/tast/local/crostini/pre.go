// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	dlcutil "chromiumos/tast/local/dlc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/timing"
)

// UnstableModels is list of models on which the Crostini tests are flaky.
// Use the guide at go/crostini-unstable-devices for when to add a device.
var UnstableModels = []string{
	// b/241027994
	"wizpig",
	"cyan",
	"edgar",
	// Platform coral
	"astronaut",
	"blacktip360",
	"blacktiplte",
	"lava",
	"nasher",
	// Platform fizz
	"jax",
	// Platform kevin
	"kevin", // crbug.com/1140145
	// Platform kukui
	"krane",
	// Platform nocturne
	"nocturne",
	// Platform reef
	"basking",
	"electro",
	// Platform trogdor
	"pompom",
	// http://b/233817342
	"lars",
	"cave",
	"chell",
	// http://b/234402067
	"taniks",
}

// CrostiniMinDiskSizeCond is a hardware condition that only runs tests on models with > 12GB of disk size.
// Crostini needs a minimum of 3GB of free space to install which is frequently not available on devices with 8GB
// disks. For more see http://crbug.com/1039403
var CrostiniMinDiskSizeCond = hwdep.MinStorage(16)

// CrostiniMinDiskSize is a hardware dependency that only runs tests
// on devices with at least 16 GB of storage.
var CrostiniMinDiskSize = hwdep.D(CrostiniMinDiskSizeCond)

// CrostiniStableCond is a hardware condition that only runs a test on models that can run Crostini tests without
// known flakiness issues.
var CrostiniStableCond = hwdep.SkipOnModel(UnstableModels...)

// CrostiniStable is a hardware dependency that only runs a test on models that can run Crostini tests without
// known flakiness issues.
var CrostiniStable = hwdep.D(CrostiniStableCond, CrostiniMinDiskSizeCond)

// CrostiniUnstableCond is a hardware condition that is the inverse of CrostiniStableCond. It only runs a test on
// models that are known to be flaky when running Crostini tests.
var CrostiniUnstableCond = hwdep.Model(UnstableModels...)

// CrostiniUnstable is a hardware dependency that is the inverse of CrostiniStable. It only runs a test on
// models that are known to be flaky when running Crostini tests.
var CrostiniUnstable = hwdep.D(CrostiniUnstableCond, CrostiniMinDiskSizeCond)

// CrostiniAppStable is a hardware dependency limiting the boards on which app testing is run.
// App testing uses a large container which needs large space. Many DUTs in the lab do not have enough space.
// The boards listed have enough space.
var CrostiniAppStable = hwdep.D(hwdep.Model("hatch", "eve", "atlas", "nami", "dragonair", "dratini", "kindred"), CrostiniMinDiskSizeCond)

// CrostiniAppUnstable is a hardware dependency in addition to CrostiniAppStable.
// These models are expected to merge with CrostiniAppStable once approved stable.
var CrostiniAppUnstable = hwdep.D(hwdep.Model(
	// hatch board.
	"akemi",
	"helios",
	"jinlon",
	"kled",
	"kohaku",
	"nightfury",
	// volteer board.
	"volteer",
	"chronicler",
	"collis",
	"copano",
	"delbin",
	"delbing",
	"drobit",
	"eldrid",
	"elemi",
	"halvor",
	"lillipup",
	"lindar",
	"malefor",
	"terrador",
	"todor",
	"trondo",
	"voema",
	"volet",
	"volta",
	"voxel",
), CrostiniMinDiskSizeCond)

// interface defined for GetInstallerOptions to allow both
// testing.State and testing.PreState to be passed in as the first
// argument.
type testingState interface {
	DataPath(string) string
}

// GetContainerMetadataArtifact gets the container metadata artifact
// for the container parameters. Note that this function will return
// different values on different architectures.
func GetContainerMetadataArtifact(debianVersion vm.ContainerDebianVersion, largeContainer bool) string {
	if largeContainer {
		return fmt.Sprintf("crostini_app_test_container_metadata_%s_%s.tar.xz", debianVersion, vm.TargetArch())
	}
	return fmt.Sprintf("crostini_test_container_metadata_%s_%s.tar.xz", debianVersion, vm.TargetArch())
}

// GetContainerRootfsArtifact gets the container rootfs artifact
// for the container parameters. Note that this function will return
// different values on different architectures.
func GetContainerRootfsArtifact(debianVersion vm.ContainerDebianVersion, largeContainer bool) string {
	if largeContainer {
		return fmt.Sprintf("crostini_app_test_container_rootfs_%s_%s.squashfs", debianVersion, vm.TargetArch())
	}
	return fmt.Sprintf("crostini_test_container_rootfs_%s_%s.squashfs", debianVersion, vm.TargetArch())
}

// GetInstallerOptions returns an InstallationOptions struct with data
// paths, and debian version set appropriately for the test.
func GetInstallerOptions(s testingState, debianVersion vm.ContainerDebianVersion, largeContainer bool, userName string) *cui.InstallationOptions {
	iOptions := &cui.InstallationOptions{
		ContainerMetadataPath: s.DataPath(GetContainerMetadataArtifact(debianVersion, largeContainer)),
		ContainerRootfsPath:   s.DataPath(GetContainerRootfsArtifact(debianVersion, largeContainer)),
		DebianVersion:         debianVersion,
		UserName:              userName,
	}

	return iOptions
}

// interface defined for GaiaLoginAvailable to allow both
// testing.State and testing.PreState to be passed in as the first
// argument.
type varState interface {
	Var(string) (string, bool)
}

// GaiaLoginAvailable returns whether or not a real gaia account is in use. This requires some variables from tast-tests-private
func GaiaLoginAvailable(s varState) bool {
	return false
}

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(crostini.PreData)
//		...
//	}
type PreData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	Container   *vm.Container
	Keyboard    *input.KeyboardEventWriter

	Post *PostTestData
}

// StartedByDlcBuster ensures that a VM running buster has
// started before the test runs. This precondition has complex
// requirements to use that are best met using the test parameter
// generator in params.go.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByDlcBuster() testing.Precondition { return startedByDlcBusterPre }

// StartedByDlcBullseye ensures that a VM running bullseye has
// started before the test runs. This precondition has complex
// requirements to use that are best met using the test parameter
// generator in params.go.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByDlcBullseye() testing.Precondition { return startedByDlcBullseyePre }

// StartedByDlcBusterGaia is similar to StartedByDlcBuster, except for
// logging in to Chrome using gaia user.
func StartedByDlcBusterGaia() testing.Precondition { return startedByDlcBusterGaiaPre }

// StartedByDlcBullseyeGaia is similar to StartedByDlcBullseye, except for
// logging in to Chrome using gaia user.
func StartedByDlcBullseyeGaia() testing.Precondition { return startedByDlcBullseyeGaiaPre }

// StartedByDlcBusterLargeContainer is similar to StartedByDlcBuster,
// but will download the large container which has apps (Gedit, Emacs, Eclipse, Android Studio, and Visual Studio) installed.
func StartedByDlcBusterLargeContainer() testing.Precondition {
	return startedByDlcBusterLargeContainerPre
}

type containerType int

const (
	normal containerType = iota
	largeContainer
)

type loginType int

const (
	loginNonGaia loginType = iota
	loginGaia
)

var startedByDlcBusterPre = &preImpl{
	name:          "crostini_started_by_dlc_buster",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	container:     normal,
	debianVersion: vm.DebianBuster,
}

var startedByDlcBullseyePre = &preImpl{
	name:          "crostini_started_by_dlc_bullseye",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	container:     normal,
	debianVersion: vm.DebianBullseye,
}

var startedByDlcBusterGaiaPre = &preImpl{
	name:          "crostini_started_by_dlc_buster_gaia",
	timeout:       chrome.GAIALoginTimeout + 7*time.Minute,
	container:     normal,
	debianVersion: vm.DebianBuster,
	loginType:     loginGaia,
}

var startedByDlcBullseyeGaiaPre = &preImpl{
	name:          "crostini_started_by_dlc_bullseye_gaia",
	timeout:       chrome.GAIALoginTimeout + 7*time.Minute,
	container:     normal,
	debianVersion: vm.DebianBullseye,
	loginType:     loginGaia,
}

var startedByDlcBusterLargeContainerPre = &preImpl{
	name:          "crostini_started_by_dlc_buster_large_container",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	container:     largeContainer,
	debianVersion: vm.DebianBuster,
}

// PostTestData contains data for post test tasks in post.go that should be persistent across tests.
type PostTestData struct {
	// Persistent reader for VM logs.
	vmLogReader *vm.LogReader
}

// Implementation of crostini's precondition.
type preImpl struct {
	name          string                    // Name of this precondition (for logging/uniqueing purposes).
	timeout       time.Duration             // Timeout for completing the precondition.
	container     containerType             // What type of container (regular or extra-large) to use.
	debianVersion vm.ContainerDebianVersion // OS version of the container image.
	cr            *chrome.Chrome
	tconn         *chrome.TestConn
	cont          *vm.Container
	keyboard      *input.KeyboardEventWriter
	startedOK     bool
	loginType     loginType

	post *PostTestData
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	// Read the -keepState variable always, to force an error if tests don't
	// have it defined.
	useLocalImage := keepState(s) && terminaDLCAvailable()

	if p.cont != nil {
		if err := BasicCommandWorks(ctx, p.cont); err != nil {
			s.Log("Precondition unsatisifed: ", err)
			p.cont = nil
			p.Close(ctx, s)
		} else if err := p.cr.Responded(ctx); err != nil {
			s.Log("Precondition unsatisfied: Chrome is unresponsive: ", err)
			p.Close(ctx, s)
		} else {
			if err := p.cr.ResetState(ctx); err != nil {
				s.Fatal("Failed to reset chrome's state: ", err)
			}
			return PreData{p.cr, p.tconn, p.cont, p.keyboard, p.post}
		}
	}

	p.post = &PostTestData{}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre
	// and copies over lxc + container boot logs.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			RunCrostiniPostTest(ctx, PreData{p.cr, p.tconn, p.cont, p.keyboard, p.post})
			p.cleanUp(ctx, s)
		}
	}()

	opts := []chrome.Option{chrome.ARCDisabled()}

	// Enable ARC++ if it is supported. We do this on every
	// supported device because some tests rely on it and this
	// lets us reduce the number of distinct preconditions. If
	// your test relies on ARC++ you should add an appropriate
	// software dependency.
	if arc.Supported() {
		if p.loginType == loginGaia {
			opts = []chrome.Option{chrome.ARCSupported()}
		} else {
			opts = []chrome.Option{chrome.ARCEnabled()}
		}
		opts = append(opts, chrome.ExtraArgs(arc.DisableSyncFlags()...))
	}
	opts = append(opts, chrome.EnableFeatures("TerminalSSH"))
	opts = append(opts, chrome.ExtraArgs("--vmodule=crostini*=1"))

	opts = append(opts, chrome.EnableFeatures("KernelnextVMs"))

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	if p.loginType == loginGaia {
		opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	}
	if useLocalImage {
		// Retain the user's cryptohome directory and previously installed VM.
		opts = append(opts, chrome.KeepState())
	}
	var err error
	if p.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, p.tconn)

	if p.keyboard, err = input.Keyboard(ctx); err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	if useLocalImage {
		s.Log("keepState attempting to start the existing VM and container by launching Terminal")
		terminalApp, err := terminalapp.Launch(ctx, p.tconn)
		if err != nil {
			s.Fatal("keepState failed to launch Terminal. Try again, cryptohome will be cleared on the next run to reset to a good state: ", err)
		}
		if err = terminalApp.Exit(p.keyboard)(ctx); err != nil {
			s.Fatal("Failed to exit Terminal window: ", err)
		}
	} else {
		// Install Crostini.
		iOptions := GetInstallerOptions(s, p.debianVersion, p.container == largeContainer, p.cr.NormalizedUser())
		if _, err := cui.InstallCrostini(ctx, p.tconn, p.cr, iOptions); err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}
	}

	p.cont, err = vm.DefaultContainer(ctx, p.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to connect to running container: ", err)
	}

	// Report disk size again after successful install.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	p.startedOK = true

	chrome.Lock()
	vm.Lock()
	shouldClose = false
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{p.cr, p.tconn, p.cont, p.keyboard, p.post}
}

// keepState returns whether the precondition should keep state from the
// previous test execution and try to recycle the VM.
func keepState(s *testing.PreState) bool {
	if str, ok := s.Var("keepState"); ok {
		b, err := strconv.ParseBool(str)
		if err != nil {
			s.Fatalf("Cannot parse argument %q to keepState: %v", str, err)
		}
		return b
	}
	return false
}

// terminaDLCAvailable returns true if the DLC package can be read.
func terminaDLCAvailable() bool {
	_, err := os.Stat("/run/imageloader/termina-dlc/package/root/vm_rootfs.img")
	return err == nil
}

// Connect connects the precondition to a running VM/container.
// If you shutdown and restart the VM you will need to call Connect again.
func (p *PreData) Connect(ctx context.Context) error {
	return p.Container.Connect(ctx, p.Chrome.NormalizedUser())
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	vm.Unlock()
	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *preImpl) cleanUp(ctx context.Context, s *testing.PreState) {
	if p.keyboard != nil {
		if err := p.keyboard.Close(); err != nil {
			s.Log("Failure closing keyboard: ", err)
		}
		p.keyboard = nil
	}

	if p.post.vmLogReader != nil {
		if err := p.post.vmLogReader.Close(); err != nil {
			s.Log("Failed to close VM log reader: ", err)
		}
	}

	// Don't uninstall crostini or delete the image for keepState so that
	// crostini is still running after the test, and the image can be reused.
	if keepState(s) && p.startedOK {
		s.Log("keepState not uninstalling Crostini and deleting image in cleanUp")
	} else {
		if p.cont != nil {
			if err := uninstallLinuxFromUI(ctx, p.tconn, p.cr); err != nil {
				s.Log("Failed to close settings window after uninstalling Linux: ", err)
			}
			p.cont = nil
		}

		// Unmount the VM image to prevent later tests from
		// using it by accident. Otherwise we may have a dlc
		// test use the component or vice versa.
		if err := dlcutil.Uninstall(ctx, "termina-dlc"); err != nil {
			s.Error("Failed to unmount termina-dlc: ", err)
		}

		if err := vm.DeleteImages(); err != nil {
			s.Log("Error deleting images: ", err)
		}
	}
	p.startedOK = false

	// Nothing special needs to be done to close the test API connection.
	p.tconn = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failure closing chrome: ", err)
		}
		p.cr = nil
	}
}

func uninstallLinuxFromUI(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Linux Settings")
	}

	// Uninstall Crostini.
	if err := st.Remove()(ctx); err != nil {
		return err
	}

	if err := st.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close settings window after uninstalling Linux")
	}
	return nil
}

// reportDiskUsage logs a report of the current disk usage.
func reportDiskUsage(ctx context.Context) error {
	var (
		statefulRoot       = "/mnt/stateful_partition"
		encryptedRoot      = filepath.Join(statefulRoot, "encrypted")
		chronosDir         = filepath.Join(encryptedRoot, "chronos")
		varDir             = filepath.Join(encryptedRoot, "var")
		encryptedBlockPath = filepath.Join(statefulRoot, "encrypted.block")
		devImageDir        = filepath.Join(statefulRoot, "dev_image")
		homeDir            = filepath.Join(statefulRoot, "home")
	)

	testing.ContextLog(ctx, "Saving disk usage snapshot")

	if err := func() error {
		outDir, ok := testing.ContextOutDir(ctx)
		if !ok {
			return errors.New("outdir not available")
		}
		f, err := os.Create(filepath.Join(outDir, "du_stateful.txt"))
		if err != nil {
			return err
		}
		defer f.Close()
		cmd := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("du -h -t 50M %s | sort -rh", shutil.Escape(statefulRoot)))
		cmd.Stdout = f
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "du %q", statefulRoot)
		}
		return nil
	}(); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Gathering disk usage data")

	fsSize := func(root string) (free, used, total uint64, err error) {
		var st unix.Statfs_t
		if err := unix.Statfs(root, &st); err != nil {
			return 0, 0, 0, err
		}
		bsz := uint64(st.Bsize)
		return st.Bfree * bsz, (st.Blocks - st.Bfree) * bsz, st.Blocks * bsz, nil
	}

	statefulFree, statefulUsed, statefulTotal, err := fsSize(statefulRoot)
	if err != nil {
		return err
	}
	encryptedFree, encryptedUsed, encryptedTotal, err := fsSize(encryptedRoot)
	if err != nil {
		return err
	}

	treeSize := func(dir string) (uint64, error) {
		out, err := testexec.CommandContext(ctx, "du", "--block-size=1", "--summarize", "--one-file-system", dir).Output(testexec.DumpLogOnError)
		if err != nil {
			return 0, errors.Wrapf(err, "du %q", dir)
		}
		ts := strings.SplitN(string(out), "\t", 2)
		if len(ts) != 2 {
			return 0, errors.Errorf("du %q: uncognized output %q", dir, string(out))
		}
		return strconv.ParseUint(ts[0], 10, 64)
	}

	chronosSize, err := treeSize(chronosDir)
	if err != nil {
		return err
	}
	varSize, err := treeSize(varDir)
	if err != nil {
		return err
	}
	encryptedBlockSize, err := treeSize(encryptedBlockPath)
	if err != nil {
		return err
	}
	devImageSize, err := treeSize(devImageDir)
	if err != nil {
		return err
	}
	homeSize, err := treeSize(homeDir)
	if err != nil {
		return err
	}

	mb := func(bytes uint64) string {
		return fmt.Sprintf("%5.1f GB", float32(bytes)/1024/1024/1024)
	}

	testing.ContextLog(ctx, "Disk usage report:")
	testing.ContextLogf(ctx, "  stateful:      %s / %s (%s free)", mb(statefulUsed), mb(statefulTotal), mb(statefulFree))
	testing.ContextLogf(ctx, "    encrypted:   %s / %s (%s free)", mb(encryptedBlockSize), mb(encryptedTotal), mb(encryptedFree))
	testing.ContextLogf(ctx, "      chronos:   %s", mb(chronosSize))
	testing.ContextLogf(ctx, "      var:       %s", mb(varSize))
	testing.ContextLogf(ctx, "      misc:      %s", mb(encryptedUsed-(chronosSize+varSize)))
	testing.ContextLogf(ctx, "      allocated: %s", mb(encryptedBlockSize-encryptedUsed))
	testing.ContextLogf(ctx, "    unencrypted: %s", mb(statefulUsed-encryptedBlockSize))
	testing.ContextLogf(ctx, "      dev_image: %s", mb(devImageSize))
	testing.ContextLogf(ctx, "      home:      %s", mb(homeSize))
	testing.ContextLogf(ctx, "      misc:      %s", mb(statefulUsed-encryptedBlockSize-(devImageSize+homeSize)))
	return nil
}
