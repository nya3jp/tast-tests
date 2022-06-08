// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui contains functions to interact with the ChromeOS parts of the crostini UI.
// This is primarily the settings and the installer.
package ui

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/crostini/lxd"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const uiTimeout = 30 * time.Second

const (
	// UninstallationTimeout is the upper limit of Install function duration.
	UninstallationTimeout = 2 * time.Minute
	// InstallationTimeout is the upper limit of Install function duration.
	InstallationTimeout = 14 * time.Minute
	// InstallCrostiniTimeout is the upper limit of InstallCrostini function duration.
	// It consists of potential uninstallation, installation and UI interactions.
	InstallCrostiniTimeout = UninstallationTimeout + InstallationTimeout + time.Minute
)

// InstallWindow is the finder for Crostini install window.
var InstallWindow = nodewith.NameRegex(regexp.MustCompile(`^Set up Linux`)).Role(role.RootWebArea)

// InstallationOptions is a struct contains parameters for Crostini installation.
type InstallationOptions struct {
	UserName              string
	ContainerMetadataPath string
	ContainerRootfsPath   string
	MinDiskSize           uint64
	DebianVersion         vm.ContainerDebianVersion
	IsSoftMinimum         bool // If true, use the maximum disk size if MinDiskSize is larger than the maximum disk size.
}

// Installer is a page object for the settings screen of the Crostini Installer.
type Installer struct {
	tconn *chrome.TestConn
}

// New creates a new Installer page object.
func New(tconn *chrome.TestConn) *Installer {
	return &Installer{tconn}
}

// SetDiskSize uses the slider on the Installer options pane to set the disk
// size to the smallest slider increment larger than the specified disk size.
// If minDiskSize is smaller than the possible minimum disk size, disk size will be the smallest size.
func (p *Installer) SetDiskSize(ctx context.Context, minDiskSize uint64, IsSoftMinimum bool) (uint64, error) {
	radioGroup := nodewith.Role(role.RadioGroup).Ancestor(InstallWindow)
	customStaticText := nodewith.Name("Custom").Role(role.StaticText).Ancestor(radioGroup)
	slider := nodewith.Role(role.Slider).Ancestor(InstallWindow)

	// Hide virtual keyboard if it appears.
	// vkb.HideVirtualKeyboard invokes Chrome API to force hide virtual keyboard.
	vkbCtx := vkb.NewContext(nil, p.tconn)
	if shown, err := vkbCtx.IsShown(ctx); err != nil && shown {
		if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
			return 0, errors.Wrap(err, "failed to hide virtual keyboard")
		}

	}

	ui := uiauto.New(p.tconn)
	if err := uiauto.Combine("click radio custom and display slider",
		ui.LeftClick(customStaticText),
		ui.FocusAndWait(slider))(ctx); err != nil {
		return 0, err
	}

	// Use keyboard to manipulate the slider rather than writing
	// custom mouse code to click on exact locations on the slider.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error in SetDiskSize: error opening keyboard")
	}
	defer kb.Close()

	defaultSize, err := settings.GetDiskSize(ctx, p.tconn, slider)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get the initial disk size")
	}
	if defaultSize == minDiskSize {
		return minDiskSize, nil
	}
	if defaultSize > minDiskSize {
		// To make sure that the final disk size is equal or larger than the minDiskSize,
		// move the slider to the left of minDiskSize first.
		minimumSize, err := settings.ChangeDiskSize(ctx, p.tconn, kb, slider, false, minDiskSize)
		if err != nil {
			return 0, errors.Wrap(err, "failed to move the disk slider to the left")
		}
		if minimumSize == minDiskSize {
			return minDiskSize, nil
		}
		if minimumSize > minDiskSize {
			testing.ContextLogf(ctx,
				"The target disk size %v is smaller than the minimum disk size, using the minimum disk size %v",
				minDiskSize, minimumSize)
			return minimumSize, nil
		}
	}

	size, err := settings.ChangeDiskSize(ctx, p.tconn, kb, slider, true, minDiskSize)
	if size < minDiskSize {
		if IsSoftMinimum {
			testing.ContextLogf(ctx, "The maximum disk size %v < the target disk size %v, using the maximum disk size %v", size, minDiskSize, size)
			return size, nil
		}
		return 0, errors.Errorf("could not set disk size to larger than %v", size)
	}
	return size, nil
}

// checkErrorMessage checks to see if an error message is currently displayed in the
// installer dialog, and returns it if one is present.
func (p *Installer) checkErrorMessage(ctx context.Context) (string, error) {
	ui := uiauto.New(p.tconn)
	statusFinder := nodewith.Role(role.Status)
	statusStaticText := nodewith.Role(role.StaticText).Ancestor(statusFinder)
	nodes, err := ui.NodesInfo(ctx, statusStaticText)
	if err != nil {
		return "", err
	}
	var messages []string
	for _, node := range nodes {
		messages = append(messages, node.Name)
	}
	message := strings.Join(messages, ": ")
	if !strings.HasPrefix(message, "Error") {
		return "", errors.Errorf("expected error message, got: %q", message)
	}
	return message, nil
}

// Install clicks the install button and waits for the Linux installation to complete.
func (p *Installer) Install(ctx context.Context) error {
	// Leave 10 seconds at the end of the context so that if the install times
	// out the context, we can still check for error messages in the installer
	// window.
	cleanupCtx := ctx
	deadline, ok := cleanupCtx.Deadline()
	if ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(cleanupCtx, deadline.Add(-10*time.Second))
		defer cancel()
	}

	// Focus on the install button to ensure virtual keyboard does not get in the
	// way and prevent the button from being clicked.
	ui := uiauto.New(p.tconn)
	installButton := nodewith.Name("Install").Role(role.Button)
	if err := uiauto.Combine("click install and wait it to finish",
		ui.LeftClick(installButton),
		ui.WithTimeout(InstallationTimeout).WaitUntilGone(InstallWindow))(ctx); err != nil {
		// If the install fails, return any error message from the installer rather than a timeout error.
		message, messageErr := p.checkErrorMessage(cleanupCtx)
		if messageErr != nil {
			testing.ContextLog(cleanupCtx, "Error checking for error message in installer: ", messageErr)
			return err
		}
		if message != "" {
			return errors.Errorf("error in installer dialog: %s", message)
		}
		return err
	}
	return nil
}

func startLxdServer(ctx context.Context, containerMetadata, containerRootfs string) (server *lxd.Server, addr string, err error) {
	server, err = lxd.NewServer(ctx, containerMetadata, containerRootfs)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to create lxd image server")
	}
	addr, err = server.ListenAndServe(ctx)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to start lxd image server")
	}

	return server, addr, nil
}

// InstallCrostini prepares image and installs Crostini from UI.
func InstallCrostini(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, iOptions *InstallationOptions) (uint64, error) {
	// Check for /dev/kvm before we do anything else.
	// On some boards in the lab the existence of this is flaky crbug.com/1072877
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return 0, errors.Wrap(err, "cannot install crostini: cannot stat /dev/kvm")
	}
	// Setup lxd server.
	server, addr, err := startLxdServer(ctx, iOptions.ContainerMetadataPath, iOptions.ContainerRootfsPath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start lxd server")
	}
	defer server.Shutdown(ctx)

	testing.ContextLog(ctx, "Installing crostini")

	url := "http://" + addr + "/"
	if err := tconn.Eval(ctx, fmt.Sprintf(
		`chrome.autotestPrivate.registerComponent(%q, %q)`,
		vm.ImageServerURLComponentName, url), nil); err != nil {
		return 0, errors.Wrap(err, "failed to run autotestPrivate.registerComponent")
	}

	if err := settings.OpenLinuxInstallerAndClickNext(ctx, tconn, cr); err != nil {
		return 0, errors.Wrap(err, "failed to launch crostini installation from Settings")
	}
	installer := New(tconn)
	var resultDiskSize uint64
	if iOptions.MinDiskSize != 0 {
		resultDiskSize, err = installer.SetDiskSize(ctx, iOptions.MinDiskSize, iOptions.IsSoftMinimum)
		if err != nil {
			return 0, errors.Wrap(err, "failed to set disk size in installation dialog")
		}
	}
	if err := installer.Install(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to install Crostini from UI")
	}

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, iOptions.UserName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to running container")
	}

	// The VM should now be running, check that all the host daemons are also running to catch any errors in our init scripts etc.
	if err = checkDaemonsRunning(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to check VM host daemons state")
	}

	if err := stopAptDaily(ctx, cont); err != nil {
		return 0, errors.Wrap(err, "failed to stop apt-daily")
	}

	if err := disableGarconPackageUpdates(ctx, cont); err != nil {
		return 0, errors.Wrap(err, "failed to stop garcon from auto-updating packages")
	}

	// If the wayland backend is used, the fonctconfig cache will be
	// generated the first time the app starts. On a low-end device, this
	// can take a long time and timeout the app executions below.
	testing.ContextLog(ctx, "Generating fontconfig cache")
	if err := cont.Command(ctx, "fc-cache").Run(testexec.DumpLogOnError); err != nil {
		return 0, errors.Wrap(err, "failed to generate fontconfig cache")
	}

	return resultDiskSize, nil
}

func expectDaemonRunning(ctx context.Context, name string) error {
	goal, state, _, err := upstart.JobStatus(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get status of job %q", name)
	}
	if goal != upstartcommon.StartGoal {
		return errors.Errorf("job %q has goal %q, want %q", name, goal, upstartcommon.StartGoal)
	}
	if state != upstartcommon.RunningState {
		return errors.Errorf("job %q has state %q, want %q", name, state, upstartcommon.RunningState)
	}
	return nil
}

func checkDaemonsRunning(ctx context.Context) error {
	if err := expectDaemonRunning(ctx, "vm_concierge"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for vm_concierge")
	}
	if err := expectDaemonRunning(ctx, "vm_cicerone"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for vm_cicerone")
	}
	if err := expectDaemonRunning(ctx, "seneschal"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for seneschal")
	}
	if err := expectDaemonRunning(ctx, "patchpanel"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for patchpanel")
	}
	if err := expectDaemonRunning(ctx, "vmlog_forwarder"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for vmlog_forwarder")
	}
	if err := expectDaemonRunning(ctx, "chunneld"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for chunneld")
	}
	if err := expectDaemonRunning(ctx, "crosdns"); err != nil {
		return errors.Wrap(err, "failed to check Daemon running for crosdns")
	}
	return nil
}

// stopAptDaily stops apt-daily systemd.
func stopAptDaily(ctx context.Context, cont *vm.Container) error {
	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		testing.ContextLogf(ctx, "Disabling service: %s", t)
		cmd := cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrapf(err, "failed to stop %s timer", t)
		}
	}
	return nil
}

// disableGarconPackageUpdates stops garcon from updating packages, which can mess with some tests.
func disableGarconPackageUpdates(ctx context.Context, cont *vm.Container) error {
	const (
		garconConfig = `DisableAutomaticCrosPackageUpdates=true
                                DisableAutomaticSecurityUpdates=true`
		configPath = ".config/cros-garcon.conf"
		localPath  = "/tmp/cros-garcon.conf"
	)
	testing.ContextLog(ctx, "Disabling garcon package updates")
	if err := ioutil.WriteFile(localPath, []byte(garconConfig), 0666); err != nil {
		return err
	}
	defer os.Remove(localPath)
	return cont.PushFile(ctx, localPath, configPath)
}
