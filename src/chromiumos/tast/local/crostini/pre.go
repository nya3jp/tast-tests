// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/timing"
)

// UnstableModels is list of models that are too flaky for the CQ. Use the standard tast
// criteria at go/tast-add-test to judge whether it should be on the CQ.
var UnstableModels = []string{
	// Platform auron
	"paine", // crbug.com/1072877
	"yuna",  // crbug.com/1072877
	// Platform banon
	"banon",
	// Platform bob
	"bob",
	// Platform buddy
	"buddy",
	// Platform celes
	"celes",
	// Platform coral
	"astronaut",
	"blacktip360",
	"blacktiplte",
	"bruce",
	"lava",
	"nasher",
	// Platform cyan
	"cyan",
	// Platform elm
	"elm",
	// Platform fiss-moblab
	"wukong",
	// Platform fizz
	"jax",
	// Platform gandof
	"gandof", // crbug.com/1072877
	// Platform grunt
	"aleena",
	"barla",
	"careena",
	"kasumi",
	"treeya",
	// Platform guado
	"guado", // crbug.com/1072877
	// Platform hana
	"hana",
	// Platform kefka
	"kefka",
	// Platform kevin
	"kevin",
	"kevin1", // crbug.com/1140145
	// Platform kukui
	"krane",
	// Platform lulu
	"lulu", // crbug.com/1072877
	// Platform nocturne
	"nocturne",
	// Platform octopus
	"ampton",
	"apel",
	"bloog",
	"bluebird",
	"bobba",
	"bobba360",
	"droid",
	"fleex",
	"foob",
	"garg",
	"laser14",
	"mimrock", // TODO: reenable once crbug.com/1101221 is fixed.
	"phaser360",
	"sparky",
	"vorticon",
	"vortininja",
	// Platform reef
	"electro",
	// Platform reks
	"reks",
	// Platform relm
	"relm",
	// Platform sarien
	"arcada",
	// Platform scarlet
	"dru",
	"dumo",
	// Platform samus
	"samus", // crbug.com/1072877
	// Platform terra
	"terra",
	// Platform tidus
	"tidus", // crbug.com/1072877
	// Platform ultima
	"ultima",
}

// CrostiniStable is a hardware dependency that only runs a test on models that can run Crostini tests without
// known flakiness issues.
var CrostiniStable = hwdep.D(hwdep.SkipOnModel(UnstableModels...))

// CrostiniUnstable is a hardware dependency that is the inverse of CrostiniStable. It only runs a test on
// models that are known to be flaky when running Crostini tests.
var CrostiniUnstable = hwdep.D(hwdep.Model(UnstableModels...))

// CrostiniAppTest is a hardware dependency limiting the boards on which app testing is run.
// App testing uses a large container which needs large space. Many DUTs in the lab do not have enough space.
// The boards listed have enough space.
var CrostiniAppTest = hwdep.D(hwdep.Platform("hatch", "eve", "atlas", "nami"))

// interface defined for GetInstallerOptions to allow both
// testing.State and testing.PreState to be passed in as the first
// argument.
type testingState interface {
	SoftwareDeps() []string
	DataPath(string) string
	Fatal(...interface{})
}

func getVMArtifact(arch string) string {
	return fmt.Sprintf("crostini_vm_%s.zip", arch)
}

func getContainerMetadataArtifact(arch string, debianVersion vm.ContainerDebianVersion, largeContainer bool) string {
	if largeContainer {
		return fmt.Sprintf("crostini_app_test_container_metadata_%s_%s.tar.xz", debianVersion, arch)
	}
	return fmt.Sprintf("crostini_test_container_metadata_%s_%s.tar.xz", debianVersion, arch)
}

func getContainerRootfsArtifact(arch string, debianVersion vm.ContainerDebianVersion, largeContainer bool) string {
	if largeContainer {
		return fmt.Sprintf("crostini_app_test_container_rootfs_%s_%s.tar.xz", debianVersion, arch)
	}
	return fmt.Sprintf("crostini_test_container_rootfs_%s_%s.tar.xz", debianVersion, arch)
}

// GetInstallerOptions returns an InstallationOptions struct with data
// paths, install mode, and debian version set appropriately for the
// test.
func GetInstallerOptions(s testingState, isComponent bool, debianVersion vm.ContainerDebianVersion, largeContainer bool) *cui.InstallationOptions {
	var arch string
	for _, dep := range s.SoftwareDeps() {
		if dep == "amd64" || dep == "arm" {
			arch = dep
		}
	}
	if arch == "" {
		s.Fatal("Running on an unknown architecture")
	}

	var mode string
	if isComponent {
		mode = cui.Artifact
	} else {
		mode = cui.Dlc
	}

	var vmPath string
	if isComponent {
		vmPath = s.DataPath(getVMArtifact(arch))
	}

	iOptions := &cui.InstallationOptions{
		VMArtifactPath:        vmPath,
		ContainerMetadataPath: s.DataPath(getContainerMetadataArtifact(arch, debianVersion, largeContainer)),
		ContainerRootfsPath:   s.DataPath(getContainerRootfsArtifact(arch, debianVersion, largeContainer)),
		Mode:                  mode,
		DebianVersion:         debianVersion,
	}

	return iOptions
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
}

// StartedByArtifactStretch ensures that a VM running stretch has
// started before the test runs. This precondition has complex
// requirements to use that are best met using the test parameter
// generator in params.go.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByArtifactStretch() testing.Precondition { return startedByArtifactStretchPre }

// StartedByArtifactBuster ensures that a VM running buster has
// started before the test runs. This precondition has complex
// requirements to use that are best met using the test parameter
// generator in params.go.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByArtifactBuster() testing.Precondition { return startedByArtifactBusterPre }

// StartedTraceVM will try to setup a debian buster VM with GPU enabled and a large disk.
func StartedTraceVM() testing.Precondition { return startedTraceVMPre }

// StartedARCEnabled is similar to StartedByArtifactBuster, but will start Chrome
// with ARCEnabled() option.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedARCEnabled() testing.Precondition { return startedARCEnabledPre }

// StartedArtifactStretchARCEnabledGaia is similar to StartedByArtifactStretch, but will
// start Chrome with ARCEnabled() option and gaia login.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedArtifactStretchARCEnabledGaia() testing.Precondition {
	return startedByArtifactStretchARCEnabledGaiaPre
}

// StartedArtifactBusterARCEnabledGaia is similar to StartedByArtifactBuster, but will
// start Chrome with ARCEnabled() option and gaia login.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedArtifactBusterARCEnabledGaia() testing.Precondition {
	return startedByArtifactBusterARCEnabledGaiaPre
}

// StartedByArtifactWithGaiaLoginStretch is similar to
// StartedByArtifactStretch, but will log in Chrome with Gaia with
// Auth() option.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByArtifactWithGaiaLoginStretch() testing.Precondition {
	return startedByArtifactWithGaiaLoginStretchPre
}

// StartedByArtifactWithGaiaLoginBuster is similar to
// StartedByArtifactBuster, but will log in Chrome with Gaia with
// Auth() option.
// Tip: Run tests with -var=keepState=true to speed up local development
func StartedByArtifactWithGaiaLoginBuster() testing.Precondition {
	return startedByArtifactWithGaiaLoginBusterPre
}

// StartedByArtifactBusterLargeContainer is similar to StartedByArtifactBuster,
// but will download the large container which has apps (Gedit, Emacs, Eclipse, Android Studio, and Visual Studio) installed.
func StartedByArtifactBusterLargeContainer() testing.Precondition {
	return startedByArtifactBusterLargeContainerPre
}

type vmSetupMode int

const (
	component vmSetupMode = iota
	dlc
)

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

var startedByArtifactStretchPre = &preImpl{
	name:          "crostini_started_by_artifact_stretch",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianStretch,
}

var startedByArtifactBusterPre = &preImpl{
	name:          "crostini_started_by_artifact_buster",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianBuster,
}

var startedTraceVMPre = &preImpl{
	name:          "crostini_started_trace_vm",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianBuster,
	minDiskSize:   16 * settings.SizeGB, // graphics.TraceReplay relies on at least 16GB size.
}

var startedARCEnabledPre = &preImpl{
	name:          "crostini_started_arc_enabled",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianBuster,
	arcEnabled:    true,
}

var startedByArtifactStretchARCEnabledGaiaPre = &preImpl{
	name:          "crostini_started_arc_enabled_stretch",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	mode:          artifact,
	debianVersion: vm.DebianStretch,
	arcEnabled:    true,
	loginType:     loginGaia, // Needs gaia login to enable Play files.
}

var startedByArtifactBusterARCEnabledGaiaPre = &preImpl{
	name:          "crostini_started_arc_enabled_buster",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	mode:          artifact,
	debianVersion: vm.DebianBuster,
	arcEnabled:    true,
	loginType:     loginGaia, // Needs gaia login to enable Play files.
}

var startedByArtifactWithGaiaLoginStretchPre = &preImpl{
	name:          "crostini_started_by_artifact_gaialogin_stretch",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianStretch,
	loginType:     loginGaia,
}

var startedByArtifactWithGaiaLoginBusterPre = &preImpl{
	name:          "crostini_started_by_artifact_gaialogin_buster",
	timeout:       chrome.LoginTimeout + 7*time.Minute,
	vmMode:        component,
	container:     normal,
	debianVersion: vm.DebianBuster,
	loginType:     loginGaia,
}

var startedByArtifactBusterLargeContainerPre = &preImpl{
	name:          "crostini_started_by_artifact_buster_large_container",
	timeout:       chrome.LoginTimeout + 10*time.Minute,
	vmMode:        component,
	container:     largeContainer,
	debianVersion: vm.DebianBuster,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name          string                    // Name of this precondition (for logging/uniqueing purposes).
	timeout       time.Duration             // Timeout for completing the precondition.
	vmMode        vmSetupMode               // Where (component/dlc) the VM comes from.
	container     containerType             // What type of container (regular or extra-large) to use.
	debianVersion vm.ContainerDebianVersion // OS version of the container image.
	arcEnabled    bool                      // Flag for whether Arc++ should be available (as well as crostini).
	minDiskSize   uint64                    // The minimum size of the VM image in bytes. 0 to use default disk size.
	cr            *chrome.Chrome
	tconn         *chrome.TestConn
	cont          *vm.Container
	keyboard      *input.KeyboardEventWriter
	loginType     loginType
	startedOK     bool
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

func (p *preImpl) writeLXCLogs(ctx context.Context) {
	vm, err := vm.GetRunningVM(ctx, p.cr.User())
	if err != nil {
		testing.ContextLog(ctx, "No running VM, possibly we failed before staring the VM")
		return
	}
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}

	testing.ContextLog(ctx, "Creating file")
	path := filepath.Join(dir, "crostini_logs.txt")
	f, err := os.Create(path)
	if err != nil {
		testing.ContextLog(ctx, "Error creating file: ", err)
		return
	}
	defer f.Close()

	f.WriteString("lxc info and lxc.log:\n")
	cmd := vm.Command(ctx, "sh", "-c", "LXD_DIR=/mnt/stateful/lxd LXD_CONF=/mnt/stateful/lxd_conf lxc info penguin --show-log")
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Run()
	if err != nil {
		testing.ContextLog(ctx, "Error getting lxc logs: ", err)
	}

	f.WriteString("\n\nconsole.log:\n")
	cmd = vm.Command(ctx, "sh", "-c", "LXD_DIR=/mnt/stateful/lxd  LXD_CONF=/mnt/stateful/lxd_conf lxc console penguin --show-log")
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Run()
	if err != nil {
		testing.ContextLog(ctx, "Error getting boot logs: ", err)
	}
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	// Read the -keepState variable always, to force an error if tests don't
	// have it defined.
	useLocalImage := keepState(s) && vm.TerminaImageExists()

	if p.cont != nil {
		if err := BasicCommandWorks(ctx, p.cont); err != nil {
			s.Log("Precondition unsatisifed: ", err)
			p.cont = nil
			p.Close(ctx, s)
		} else if err := p.cr.Responded(ctx); err != nil {
			s.Log("Precondition unsatisfied: Chrome is unresponsive: ", err)
			p.Close(ctx, s)
		} else {
			return p.buildPreData(ctx, s)
		}
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre
	// and copies over lxc + container boot logs.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.writeLXCLogs(ctx)
			if p.cont != nil {
				TrySaveVMLogs(ctx, p.cont.VM)
			}
			p.cleanUp(ctx, s)
		}
	}()

	opts := []chrome.Option{chrome.ARCDisabled()}
	if p.arcEnabled {
		if p.loginType == loginGaia {
			opts = []chrome.Option{chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)}
		} else {
			opts = []chrome.Option{chrome.ARCEnabled()}
		}
	}
	opts = append(opts, chrome.ExtraArgs("--vmodule=crostini*=1"))

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	if p.loginType == loginGaia {
		opts = append(opts, chrome.Auth(
			s.RequiredVar("crostini.gaiaUsername"),
			s.RequiredVar("crostini.gaiaPassword"),
			s.RequiredVar("crostini.gaiaID"),
		), chrome.GAIALogin())
	}
	if p.vmMode == dlc {
		opts = append(opts, chrome.EnableFeatures("CrostiniUseDlc"))
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
		if err = terminalApp.Exit(ctx, p.keyboard); err != nil {
			s.Fatal("Failed to exit Terminal window: ", err)
		}
	} else {
		// Install Crostini.
		iOptions := GetInstallerOptions(s, p.vmMode == component, p.debianVersion, p.container == largeContainer)
		iOptions.UserName = p.cr.User()
		iOptions.MinDiskSize = p.minDiskSize
		if _, err := cui.InstallCrostini(ctx, p.tconn, iOptions); err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}
	}

	p.cont, err = vm.DefaultContainer(ctx, p.cr.User())
	if err != nil {
		s.Fatal("Failed to connect to running container: ", err)
	}

	// Report disk size again after successful install.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	ret := p.buildPreData(ctx, s)
	p.startedOK = true

	chrome.Lock()
	vm.Lock()
	shouldClose = false
	return ret
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

// Connect connects the precondition to a running VM/container.
// If you shutdown and restart the VM you will need to call Connect again.
func (p *PreData) Connect(ctx context.Context) error {
	return p.Container.Connect(ctx, p.Chrome.User())
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

	// Don't stop concierge or delete the image for keepState so that
	// crostini is still running after the test, and the image can be reused.
	if keepState(s) && p.startedOK {
		s.Log("keepState not stopping concierge or unmounting and deleting image in cleanUp")
	} else {
		if p.cont != nil {
			if err := vm.StopConcierge(ctx); err != nil {
				s.Log("Failure stopping concierge: ", err)
			}
			p.cont = nil
		}
		// It is always safe to unmount the component, which just posts some
		// logs if it was never mounted.
		vm.UnmountComponent(ctx)
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

// buildPreData is a helper method that resets the machine state in
// advance of building the precondition data for the actual tests.
func (p *preImpl) buildPreData(ctx context.Context, s *testing.PreState) PreData {
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{p.cr, p.tconn, p.cont, p.keyboard}
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
