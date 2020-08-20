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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/crostini/lxd"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
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
	"auron_paine",
	"auron_yuna",
	// Platform banon
	"banon",
	// Platform bob
	"bob",
	// Platform buddy
	"buddy",
	// Platform celes
	"celes",
	// Platform coral
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
	"gandof",
	// Platform grunt
	"aleena",
	"barla",
	"careena",
	"kasumi",
	"treeya",
	// Platform guado
	"guado",
	// Platform hana
	"hana",
	// Platform kefka
	"kefka",
	// Platform kevin
	"kevin",
	// Platform kukui
	"krane",
	// Platform lulu
	"lulu",
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
	// Platform relm
	"relm",
	// Platform sarien
	"arcada",
	// Platform scarlet
	"dru",
	"dumo",
	// Platform terra
	"terra",
	// Platform ultima
	"ultima",
}

// CrostiniStable is a hardware dependency that only runs a test on models that can run Crostini tests without
// known flakiness issues.
var CrostiniStable = hwdep.D(hwdep.SkipOnModel(UnstableModels...))

// CrostiniUnstable is a hardware dependency that is the inverse of CrostiniStable. It only runs a test on
// models that are known to be flaky when running Crostini tests.
var CrostiniUnstable = hwdep.D(hwdep.Model(UnstableModels...))

// ImageArtifact holds the name of the artifact which will be used to
// boot crostini. When using the StartedByArtifact precondition, you
// must list this as one of the data dependencies of your test.
const ImageArtifact string = "crostini_guest_images.tar"

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

// StartedByArtifact is similar to StartedByDownloadBuster, but will
// use a pre-built image as a data-dependency rather than downloading one. To
// use this precondition you must have crostini.ImageArtifact as a data dependency.
func StartedByArtifact() testing.Precondition { return startedByArtifactPre }

// StartedByDownloadStretch is a precondition that ensures a tast test
// will begin after crostini has been started by downloading an image
// running Debian Stretch.
func StartedByDownloadStretch() testing.Precondition { return startedByDownloadStretchPre }

// StartedByDownloadBuster is a precondition that ensures a tast test will
// begin after crostini has been started by downloading an image
// running Debian Buster.
func StartedByDownloadBuster() testing.Precondition { return startedByDownloadBusterPre }

// StartedTraceVM will try to setup a debian buster VM with GPU enabled and a large disk.
func StartedTraceVM() testing.Precondition { return startedTraceVMPre }

// StartedARCEnabled is similar to StartedByArtifact, but will start Chrome
// with ARCEnabled() option.
func StartedARCEnabled() testing.Precondition { return startedARCEnabledPre }

// StartedByArtifactWithGaiaLogin is similar to StartedByArtifact, but will log in Chrome with Gaia
// with Auth() option.
func StartedByArtifactWithGaiaLogin() testing.Precondition { return startedByArtifactWithGaiaLoginPre }

// StartedByDownloadBusterWithGaiaLogin is similar to StartedByDownloadBuster, but will log in Chrome with Gaia
// with Auth() option.
func StartedByDownloadBusterWithGaiaLogin() testing.Precondition {
	return startedByDownloadBusterWithGaiaLoginPre
}

type setupMode int

const (
	artifact setupMode = iota
	download
)

type loginType int

const (
	loginNonGaia loginType = iota
	loginGaia
)

var startedByArtifactPre = &preImpl{
	name:    "crostini_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    artifact,
}

var startedByDownloadStretchPre = &preImpl{
	name:    "crostini_started_by_download_stretch",
	timeout: chrome.LoginTimeout + 10*time.Minute,
	mode:    download,
	arch:    vm.DebianStretch,
}

var startedByDownloadBusterPre = &preImpl{
	name:    "crostini_started_by_download_buster",
	timeout: chrome.LoginTimeout + 10*time.Minute,
	mode:    download,
	arch:    vm.DebianBuster,
}

var startedTraceVMPre = &preImpl{
	name:        "crostini_started_trace_vm",
	timeout:     chrome.LoginTimeout + 10*time.Minute,
	mode:        artifact,
	minDiskSize: 16 * cui.SizeGB, // graphics.TraceReplay relies on at least 16GB size.
}

var startedARCEnabledPre = &preImpl{
	name:       "crostini_started_arc_enabled",
	timeout:    chrome.LoginTimeout + 10*time.Minute,
	mode:       artifact,
	arcEnabled: true,
}

var startedByArtifactWithGaiaLoginPre = &preImpl{
	name:      "crostini_started_by_artifact_gaialogin",
	timeout:   chrome.LoginTimeout + 7*time.Minute,
	mode:      artifact,
	loginType: loginGaia,
}

var startedByDownloadBusterWithGaiaLoginPre = &preImpl{
	name:      "crostini_started_by_download_buster_gaialogin",
	timeout:   chrome.LoginTimeout + 10*time.Minute,
	mode:      download,
	arch:      vm.DebianBuster,
	loginType: loginGaia,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name        string               // Name of this precondition (for logging/uniqueing purposes).
	timeout     time.Duration        // Timeout for completing the precondition.
	mode        setupMode            // Where (download/build artifact) the container image comes from.
	arch        vm.ContainerArchType // Architecture/distribution of the container image.
	arcEnabled  bool                 // Flag for whether Arc++ should be available (as well as crostini).
	minDiskSize uint64               // The minimum size of the VM image in bytes. 0 to use default disk size.
	cr          *chrome.Chrome
	tconn       *chrome.TestConn
	cont        *vm.Container
	keyboard    *input.KeyboardEventWriter
	loginType   loginType
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

func expectDaemonRunning(ctx context.Context, name string) error {
	goal, state, _, err := upstart.JobStatus(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get status of job %q", name)
	}
	if goal != upstart.StartGoal {
		return errors.Errorf("job %q has goal %q, expected %q", name, goal, upstart.StartGoal)
	}
	if state != upstart.RunningState {
		return errors.Errorf("job %q has state %q, expected %q", name, state, upstart.RunningState)
	}
	return nil
}

func checkDaemonsRunning(ctx context.Context) error {
	if err := expectDaemonRunning(ctx, "vm_concierge"); err != nil {
		return err
	}
	if err := expectDaemonRunning(ctx, "vm_cicerone"); err != nil {
		return err
	}
	if err := expectDaemonRunning(ctx, "seneschal"); err != nil {
		return err
	}
	if err := expectDaemonRunning(ctx, "patchpanel"); err != nil {
		return err
	}
	if err := expectDaemonRunning(ctx, "vmlog_forwarder"); err != nil {
		return err
	}
	return nil
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cont != nil {
		if err := SimpleCommandWorks(ctx, p.cont); err != nil {
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

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

	opt := chrome.ARCDisabled()
	if p.arcEnabled {
		opt = chrome.ARCEnabled()
	}

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	opts := []chrome.Option{opt, chrome.ExtraArgs("--vmodule=crostini*=1")}
	if p.loginType == loginGaia {
		opts = append(opts, chrome.Auth(
			s.RequiredVar("crostini.gaiaUsername"),
			s.RequiredVar("crostini.gaiaPassword"),
			s.RequiredVar("crostini.gaiaID"),
		), chrome.GAIALogin())
	}
	if FastDebug && vm.TerminaImageExists() {
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

	if FastDebug && vm.TerminaImageExists() {
		s.Log("FastDebug attempting to start the existing VM and container")
		if err := p.fastDebugStart(ctx, s.OutDir()); err != nil {
			// Delete TerminaImage so that the next run will clear cryptohome and reset to a good state.
			vm.UnmountComponent(ctx)
			if err := vm.DeleteImages(); err != nil {
				s.Fatalf("FastDebug failed and could not reset. You must manually delete %s to reset: %v", vm.TerminaImage, err)
			}
			s.Fatal("FastDebug failed. Try again, cryptohome will be cleared on the next run to reset to a good state")
		}
	} else {
		// Download images and run the GUI installer.
		terminaImage := ""
		containerDir := ""

		switch p.mode {
		case download:
			terminaImage, err = vm.DownloadStagingTermina(ctx)
			if err != nil {
				s.Fatal("Failed to download staging termina: ", err)
			}

			containerDir, err = vm.DownloadStagingContainer(ctx, p.arch)
			if err != nil {
				s.Fatal("Failed to download staging container: ", err)
			}

		case artifact:
			artifactPath := s.DataPath(ImageArtifact)
			terminaImage, err = vm.ExtractTermina(ctx, artifactPath)
			if err != nil {
				s.Fatal("Failed to extract termina: ", err)
			}

			containerDir, err = vm.ExtractContainer(ctx, p.cr.User(), artifactPath)
			if err != nil {
				s.Fatal("Failed to extract container: ", err)
			}
		default:
			s.Fatal("Unrecognized mode: ", p.mode)
		}

		server, err := lxd.NewServer(ctx, containerDir)
		if err != nil {
			s.Fatal("Error creating lxd image server: ", err)
		}
		addr, err := server.ListenAndServe(ctx)
		if err != nil {
			s.Fatal("Error starting lxd image server: ", err)
		}
		defer server.Shutdown(ctx)

		s.Log("Installing crostini")

		url := "http://" + addr + "/"
		if err := p.tconn.Eval(ctx, fmt.Sprintf(
			`chrome.autotestPrivate.registerComponent(%q, %q)`,
			vm.ImageServerURLComponentName, url), nil); err != nil {
			s.Fatal("Failed to run autotestPrivate.registerComponent: ", err)
		}

		s.Log("MountComponent ", terminaImage)
		vm.MountComponent(ctx, terminaImage)
		if err := p.tconn.Eval(ctx, fmt.Sprintf(
			`chrome.autotestPrivate.registerComponent(%q, %q)`,
			vm.TerminaComponentName, vm.TerminaMountDir), nil); err != nil {
			s.Fatal("Failed to run autotestPrivate.registerComponent: ", err)
		}

		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, p.tconn)

		settings, err := cui.OpenSettings(ctx, p.tconn)
		if err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}
		installer, err := settings.OpenInstaller(ctx)
		if err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}
		if p.minDiskSize != 0 {
			if err := installer.SetDiskSize(ctx, p.minDiskSize); err != nil {
				s.Fatal("SetDiskSize error: ", err)
			}
		}
		if err := installer.Install(ctx); err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}

		// Report disk size again after successful install.
		if err := reportDiskUsage(ctx); err != nil {
			s.Log("Failed to gather disk usage: ", err)
		}
	}

	if err := p.Connect(ctx); err != nil {
		s.Fatal("Error connecting to running container: ", err)
	}

	// The VM should now be running, check that all the host daemons are also running to catch any errors in our init scripts etc.
	if err = checkDaemonsRunning(ctx); err != nil {
		s.Fatal("VM host daemons in an unexpected state: ", err)
	}

	if p.keyboard, err = input.Keyboard(ctx); err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		s.Log("Disabling service: ", t)
		cmd := p.cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to stop %s timer: %v", t, err)
		}
	}

	ret := p.buildPreData(ctx, s)

	chrome.Lock()
	vm.Lock()
	shouldClose = false
	return ret
}

// Connect connects the precondition to a running VM/container.
// If you shutdown and restart the VM you will need to call Connect again.
func (p *preImpl) Connect(ctx context.Context) error {
	var err error
	p.cont, err = vm.DefaultContainer(ctx, p.cr.User())
	return err
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

	if p.cont != nil {
		if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Log("Failure dumping container log: ", err)
		}
		if err := vm.StopConcierge(ctx); err != nil {
			s.Log("Failure stopping concierge: ", err)
		}
		p.cont = nil
	}
	// It is always safe to unmount the component, which just posts some
	// logs if it was never mounted.
	vm.UnmountComponent(ctx)
	// Don't delete the image for FastDebug so that it can be used in the next run.
	if FastDebug {
		s.Log("FastDebug not deleting image in cleanUp")
	} else {
		if err := vm.DeleteImages(); err != nil {
			s.Log("Error deleting images: ", err)
		}
	}

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

// fastDebugStart attempts to start the default VM and container if they
// already exist on the device from a previous test run.
func (p *preImpl) fastDebugStart(ctx context.Context, dir string) error {
	if err := vm.MountComponent(ctx, vm.TerminaImage); err != nil {
		return err
	}
	if err := p.tconn.Eval(ctx, fmt.Sprintf(
		`chrome.autotestPrivate.registerComponent(%q, %q)`,
		vm.TerminaComponentName, vm.TerminaMountDir), nil); err != nil {
		return errors.Wrap(err, "failed to run autotestPrivate.registerComponent: ", err)
	}
	concierge, err := vm.NewConcierge(ctx, p.cr.User())
	if err != nil {
		return err
	}
	vmInstance := vm.NewDefaultVM(concierge, false, vm.DefaultDiskSize)
	if err := vmInstance.Start(ctx); err != nil {
		return err
	}
	c, err := vm.DefaultContainer(ctx, p.cr.User())
	if err != nil {
		return err
	}
	if err = c.StartAndWait(ctx, dir); err != nil {
		return err
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
