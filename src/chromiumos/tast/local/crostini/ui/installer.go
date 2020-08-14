// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui contains functions to interact with the ChromeOS parts of the crostini UI.
// This is primarily the settings and the installer.
package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	"chromiumos/tast/local/crostini/lxd"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	// SizeB is a multiplier to convert bytes to bytes.
	SizeB = 1
	// SizeKB is a multiplier to convert bytes to kilobytes.
	SizeKB = 1024
	// SizeMB is a multiplier to convert bytes to megabytes.
	SizeMB = 1024 * 1024
	// SizeGB is a multiplier to convert bytes to gigabytes.
	SizeGB = 1024 * 1024 * 1024
	// SizeTB is a multiplier to convert bytes to terabytes.
	SizeTB = 1024 * 1024 * 1024 * 1024
)

const uiTimeout = 30 * time.Second

// Image setup mode.
const (
	Artifact = "artifact"
	Download = "download"
)

// InstallationOptions is a struct contains parameters for Crostini installation.
type InstallationOptions struct {
	UserName          string
	Mode              string
	ImageArtifactPath string
	MinDiskSize       uint64
	Arch              vm.ContainerArchType
}

// Installer is a page object for the settings screen of the Crostini Installer.
type Installer struct {
	tconn *chrome.TestConn
}

func parseDiskSizeString(str string) (uint64, error) {
	parts := strings.Split(str, " ")
	if len(parts) != 2 {
		return 0, errors.Errorf("could not parseDiskSizeString %s: does not have exactly 2 space separated parts", str)
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, errors.Wrapf(err, "could not parseDiskSizeString %s", str)
	}
	unitMap := map[string]float64{
		"B":  SizeB,
		"KB": SizeKB,
		"MB": SizeMB,
		"GB": SizeGB,
		"TB": SizeTB,
	}
	units, ok := unitMap[parts[1]]
	if !ok {
		return 0, errors.Errorf("could not parseDiskSizeString %s: does not have a recognized units string", str)
	}
	return uint64(num * units), nil
}

// SetDiskSize uses the slider on the Installer options pane to set the disk
// size to the smallest slider increment larger than the specified disk size.
func (p *Installer) SetDiskSize(ctx context.Context, minDiskSize uint64) error {
	// TODO: The name only applies to chromebook but not chromebox. Parse also string for Chromebox.
	window := uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeRootWebArea, Name: "Set up Linux (Beta) on your Chromebook"}, uiTimeout)
	radioGroup := window.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeRadioGroup}, uiTimeout)
	slider := window.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeSlider}, uiTimeout)

	if err := uig.Do(ctx, p.tconn, uig.Steps(
		radioGroup.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText, Name: "Custom"}, uiTimeout).LeftClick(),
		slider.FocusAndWait(uiTimeout),
	)); err != nil {
		return errors.Wrap(err, "error in SetDiskSize()")
	}

	// Use keyboard to manipulate the slider rather than writing
	// custom mouse code to click on exact locations on the slider.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error in SetDiskSize: error opening keyboard")
	}
	defer kb.Close()

	// getSize returns the current size based on the slider text.
	getSize := func() (uint64, error) {
		node, err := uig.GetNode(ctx, p.tconn, slider.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText}, uiTimeout))
		if err != nil {
			return 0, errors.Wrap(err, "error getting disk size setting")
		}
		defer node.Release(ctx)
		return parseDiskSizeString(node.Name)
	}

	for {
		size, err := getSize()
		if err != nil {
			return errors.Wrap(err, "error getting disk size")
		}
		if size >= minDiskSize {
			break
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := kb.Accel(ctx, "right"); err != nil {
				return errors.Wrap(err, "error sending right arrow key")
			}
			curSize, err := getSize()
			if err != nil {
				return errors.Wrap(err, "error getting disk size")
			}
			if size == curSize {
				return errors.Errorf("could not set disk size to larger than %v", curSize)
			}
			return nil
		}, &testing.PollOptions{Interval: 50 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
			return err
		}
	}
	return nil
}

// Install clicks the install button and waits for the Linux installation to complete.
func (p *Installer) Install(ctx context.Context) error {
	// First check for an error screen.
	status, err := ui.Find(ctx, p.tconn, ui.FindParams{Role: ui.RoleTypeStatus})
	if err == nil {
		defer status.Release(ctx)
		// There is an error message, fetch and return it rather than the "can't find Install button" error.
		nodes, err := status.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeStaticText})
		if err != nil {
			return err
		}
		var messages []string
		for _, node := range nodes {
			messages = append(messages, node.Name)
			node.Release(ctx)
		}
		message := strings.Join(messages, ": ")
		if strings.HasPrefix(message, "Error") {
			return errors.Errorf("error message in dialog: %s", message)
		}
	}
	// Focus on the install button to ensure virtual keyboard does not get in the
	// way and prevent the button from being clicked.
	install := uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Install"}, uiTimeout)
	return uig.Do(ctx, p.tconn,
		uig.Steps(
			install.FocusAndWait(uiTimeout),
			install.LeftClick(),
			uig.WaitUntilDescendantGone(ui.FindParams{Role: ui.RoleTypeButton, Name: "Cancel"}, 10*time.Minute)).WithNamef("Install()"))
}

func prepareImages(ctx context.Context, iOptions *InstallationOptions) (containerDir, terminaImage string, err error) {
	// Prepare image.
	switch iOptions.Mode {
	case Download:
		terminaImage, err = vm.DownloadStagingTermina(ctx)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to download staging termina")
		}

		containerDir, err = vm.DownloadStagingContainer(ctx, iOptions.Arch)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to download staging container")
		}

	case Artifact:
		terminaImage, err = vm.ExtractTermina(ctx, iOptions.ImageArtifactPath)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to extract termina: ")
		}

		containerDir, err = vm.ExtractContainer(ctx, iOptions.UserName, iOptions.ImageArtifactPath)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to extract container: ")
		}
	default:
		return "", "", errors.Errorf("unrecognized mode: %q", iOptions.Mode)
	}
	return containerDir, terminaImage, nil
}

func startLxdServer(ctx context.Context, containerDir string) (server *lxd.Server, addr string, err error) {
	server, err = lxd.NewServer(ctx, containerDir)
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
func InstallCrostini(ctx context.Context, tconn *chrome.TestConn, iOptions *InstallationOptions) error {
	// Setup lxd server.
	containerDir, terminaImage, err := prepareImages(ctx, iOptions)
	if err != nil {
		return errors.Wrap(err, "failed to prepare image")
	}
	server, addr, err := startLxdServer(ctx, containerDir)
	if err != nil {
		return errors.Wrap(err, "failed to start lxd server")
	}
	defer server.Shutdown(ctx)

	testing.ContextLog(ctx, "Installing crostini")

	url := "http://" + addr + "/"
	if err := tconn.Eval(ctx, fmt.Sprintf(
		`chrome.autotestPrivate.registerComponent(%q, %q)`,
		vm.ImageServerURLComponentName, url), nil); err != nil {
		return errors.Wrap(err, "failed to run autotestPrivate.registerComponent")
	}

	vm.MountComponent(ctx, terminaImage)
	if err := tconn.Eval(ctx, fmt.Sprintf(
		`chrome.autotestPrivate.registerComponent(%q, %q)`,
		vm.TerminaComponentName, vm.TerminaMountDir), nil); err != nil {
		return errors.Wrap(err, "failed to run autotestPrivate.registerComponent")
	}

	// Install Crostini from Settings.
	settings, err := settings.Open(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Settings")
	}
	defer settings.Close(ctx)

	if err := settings.OpenInstaller(ctx); err != nil {
		return errors.Wrap(err, "failed to launch crostini installation from Settings")
	}
	installer := &Installer{tconn}
	if iOptions.MinDiskSize != 0 {
		if err := installer.SetDiskSize(ctx, iOptions.MinDiskSize); err != nil {
			return errors.Wrap(err, "failed to set disk size in installation dialog")
		}
	}
	if err := installer.Install(ctx); err != nil {
		return errors.Wrap(err, "failed to install Crostini from UI")
	}

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, iOptions.UserName)
	if err != nil {
		return errors.Wrap(err, "failed to connect to running container")
	}

	// The VM should now be running, check that all the host daemons are also running to catch any errors in our init scripts etc.
	if err = checkDaemonsRunning(ctx); err != nil {
		return errors.Wrap(err, "failed to check VM host daemons state")
	}

	if err := stopAptDaily(ctx, cont); err != nil {
		return errors.Wrap(err, "failed to stop apt-daily")
	}

	// If the wayland backend is used, the fonctconfig cache will be
	// generated the first time the app starts. On a low-end device, this
	// can take a long time and timeout the app executions below.
	testing.ContextLog(ctx, "Generating fontconfig cache")
	if err := cont.Command(ctx, "fc-cache").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to generate fontconfig cache")
	}

	return nil
}

func expectDaemonRunning(ctx context.Context, name string) error {
	goal, state, _, err := upstart.JobStatus(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get status of job %q", name)
	}
	if goal != upstart.StartGoal {
		return errors.Errorf("job %q has goal %q, want %q", name, goal, upstart.StartGoal)
	}
	if state != upstart.RunningState {
		return errors.Errorf("job %q has state %q, want %q", name, state, upstart.RunningState)
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
			return errors.Wrapf(err, "failed to stop %s timer: %v", t, err)
		}
	}
	return nil
}
