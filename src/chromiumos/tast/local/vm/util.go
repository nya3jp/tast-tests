// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/compupdater"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// TerminaComponentName is the name of the Chrome component for the VM kernel and rootfs.
	TerminaComponentName = "cros-termina"

	// TerminaMountDir is a path to the location where we will mount the termina component.
	TerminaMountDir = "/run/imageloader/cros-termina/99999.0.0"

	terminaComponentDownloadPath     = "/usr/local/cros-termina"
	terminaComponentStagingURLFormat = "https://storage.googleapis.com/termina-component-testing/%d/staging"
	terminaComponentURLFormat        = "https://storage.googleapis.com/termina-component-testing/%d/%s/chromeos_%s-archive/files.zip"
	lsbReleasePath                   = "/etc/lsb-release"
	milestoneKey                     = "CHROMEOS_RELEASE_CHROME_MILESTONE"
)

// ComponentType represents the VM component type.
type ComponentType int

const (
	// ComponentUpdater indicates that the live component should be fetched from the component updater service.
	ComponentUpdater ComponentType = iota
	// StagingComponent indicates that the current staging component should be fetched from the GS component testing bucket.
	StagingComponent
)

// downloadComponent downloads a component with the given version string.
// Returns the path to the image that holds the component.
func downloadComponent(ctx context.Context, milestone int, version string) (string, error) {
	componentDir := filepath.Join(terminaComponentDownloadPath, version)
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return "", err
	}
	imagePath := filepath.Join(componentDir, "image.ext4")
	if _, err := os.Stat(imagePath); err != nil {
		if !os.IsNotExist(err) {
			// Something failed other than the image not existing.
			return "", errors.Wrap(err, "failed to stat image.ext4")
		}
	} else {
		// The image exists, so go ahead and use it.
		return imagePath, nil
	}

	// Build the URL for the component, which depends on the DUT's arch.
	var componentArch string
	if runtime.GOARCH == "amd64" {
		componentArch = "intel64"
	} else {
		componentArch = "arm32"
	}

	// Download the files.zip from the component GS bucket.
	url := fmt.Sprintf(terminaComponentURLFormat, milestone, version, componentArch)
	testing.ContextLogf(ctx, "Downloading VM component version %s from: %s", version, url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("component download failed: %s", resp.Status)
	}
	filesPath := filepath.Join(componentDir, "files.zip")
	filesZip, err := os.Create(filesPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(filesZip, resp.Body); err != nil {
		filesZip.Close()
		os.Remove(filesPath)
		return "", err
	}
	filesZip.Close()

	// Extract the zip. We expect an image.ext4 file in the output.
	unzipCmd := testexec.CommandContext(ctx, "unzip", filesPath, "image.ext4", "-d", componentDir)
	if err := unzipCmd.Run(); err != nil {
		unzipCmd.DumpLog(ctx)
		return "", errors.Wrap(err, "failed to unzip")
	}
	return imagePath, nil
}

// MountArtifactComponent extracts and mounts the VM image from build artifacts.
func MountArtifactComponent(ctx context.Context, artifactPath string) error {
	componentDir := filepath.Join(terminaComponentDownloadPath, "artifact")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return err
	}

	imagePath := filepath.Join(componentDir, "image.ext4")
	if _, err := os.Stat(imagePath); err == nil {
		// The image exists, so go ahead and use it.
		return mountComponent(ctx, imagePath)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Extract just the VM image from the tarball.
	cmd := testexec.CommandContext(ctx, "tar", "xvf", artifactPath, "-C", componentDir, "vm_image.zip")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to untar")
	}

	zipPath := filepath.Join(componentDir, "vm_image.zip")
	// Extract the zip. We expect an image.ext4 file in the output.
	unzipCmd := testexec.CommandContext(ctx, "unzip", zipPath, "image.ext4", "-d", componentDir)
	if err := unzipCmd.Run(); err != nil {
		unzipCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to unzip")
	}
	return mountComponent(ctx, imagePath)
}

// mountComponent mounts a component image from the provided image path.
func mountComponent(ctx context.Context, image string) error {
	if err := os.MkdirAll(TerminaMountDir, 0755); err != nil {
		return err
	}
	// Unmount any existing component.
	unix.Unmount(TerminaMountDir, 0)

	// We could call losetup manually and use the mount syscall... or
	// we could let mount(8) do the work.
	mountCmd := testexec.CommandContext(ctx, "mount", image, "-o", "loop", TerminaMountDir)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to mount component")
	}

	return nil
}

func mountComponentUpdater(ctx context.Context) error {
	updater, err := compupdater.New(ctx)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Mounting %q component", TerminaComponentName)
	resp, err := updater.LoadComponent(ctx, TerminaComponentName, compupdater.Mount)
	if err != nil {
		return errors.Wrapf(err, "mounting %q component failed", TerminaComponentName)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	// Ensure that the 99999.0.0 component isn't used.
	// Unmount any existing component and delete the 99999.0.0 directory.
	unix.Unmount(TerminaMountDir, 0)
	return os.RemoveAll(TerminaMountDir)
}

// SetUpComponent sets up the VM component according to the specified ComponentType.
func SetUpComponent(ctx context.Context, c ComponentType) error {
	if c == ComponentUpdater {
		return mountComponentUpdater(ctx)
	}

	milestone, err := getMilestone()
	if err != nil {
		return err
	}

	var url string
	switch c {
	case StagingComponent:
		url = fmt.Sprintf(terminaComponentStagingURLFormat, milestone)
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("component symlink download failed: %s", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(body))

	imagePath, err := downloadComponent(ctx, milestone, version)
	if err != nil {
		return err
	}

	return mountComponent(ctx, imagePath)
}

// UnmountComponent unmounts any active VM component.
func UnmountComponent(ctx context.Context) {
	if err := unix.Unmount(TerminaMountDir, 0); err != nil {
		testing.ContextLog(ctx, "Failed to unmount component: ", err)
	}

	if err := os.Remove(TerminaMountDir); err != nil {
		testing.ContextLog(ctx, "Failed to remove component mount directory: ", err)
	}
}

// getMilestone returns the Chrome OS milestone for this build.
func getMilestone() (int, error) {
	f, err := os.Open(lsbReleasePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), "=")
		if len(s) != 2 {
			continue
		}
		if s[0] == milestoneKey {
			val, err := strconv.Atoi(s[1])
			if err != nil {
				return 0, errors.Wrapf(err, "%q is not a valid milestone number", s[1])
			}
			return val, nil
		}
	}
	return 0, errors.New("no milestone key in lsb-release file")
}

// EnableCrostini sets the preference for Crostini being enabled as this is required for
// some of the Chrome integration tests to function properly.
func EnableCrostini(ctx context.Context, tconn *chrome.Conn) error {
	if err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.setCrostiniEnabled(true, () => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, nil); err != nil {
		return errors.Wrap(err, "running autotestPrivate.setCrostiniEnabled failed")
	}
	return nil
}

// waitForDBusSignal waits on a SignalWatcher and returns the unmarshaled signal. |optSpec| matches a subset of the watching signals if |watcher|
// listens on multiple signals. Pass nil if we want to wait for any signal matches by |watcher|.
func waitForDBusSignal(ctx context.Context, watcher *dbusutil.SignalWatcher, optSpec *dbusutil.MatchSpec, sigResult proto.Message) error {
	for {
		select {
		case sig := <-watcher.Signals:
			if optSpec == nil || optSpec.MatchesSignal(sig) {
				if len(sig.Body) == 0 {
					return errors.New("signal lacked a body")
				}
				buf, ok := sig.Body[0].([]byte)
				if !ok {
					return errors.New("signal body is not a byte slice")
				}
				if err := proto.Unmarshal(buf, sigResult); err != nil {
					return errors.Wrap(err, "failed unmarshaling signal body")
				}
				return nil
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "didn't get D-Bus signal")
		}
	}
}

// findIPv4 returns the first IPv4 address found in a space separated list of IPs.
func findIPv4(ips string) (string, error) {
	for _, v := range strings.Fields(ips) {
		ip := net.ParseIP(v)
		if ip != nil && ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", errors.Errorf("could not find IPv4 address in %q", ips)
}

// CreateDefaultVMContainer prepares a VM and container with default settings and
// either the live or staging container versions. The directory dir may be used
// to store logs on failure. If the container type is Tarball, then artifactPath
// must be specified with the path to the tarball containing the termina VM
// and container. Otherwise, artifactPath is ignored. If enableGPU is set, it will
// pass it to VM to force gpu enabled.
func CreateDefaultVMContainer(ctx context.Context, dir, user string, t ContainerType, artifactPath string, enableGPU bool) (*Container, error) {
	vmInstance, err := CreateDefaultVM(ctx, dir, user, t, artifactPath, enableGPU)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create default VM instance")
	}

	container, err := CreateDefaultContainer(ctx, vmInstance, t, dir)
	if err != nil {
		// Stopping Concierge should also dispose vmInstance.
		if stopErr := StopConcierge(ctx); stopErr != nil {
			testing.ContextLog(ctx, "Failed to stop concierge: ", stopErr)
		}
		return nil, errors.Wrap(err, "failed to create default Container")
	}
	return container, nil
}

// RestartDefaultVMContainer restarts a VM and container that were previously shut down.
func RestartDefaultVMContainer(ctx context.Context, dir string, container *Container) error {
	if err := container.VM.Start(ctx); err != nil {
		return err
	}
	if err := container.StartAndWait(ctx, dir); err != nil {
		return err
	}
	return nil
}

// CreateVSHCommand creates a command to be run in a VM over vsh. The command
// parameter is required followed by an optional variatic list of strings as
// args. The command object is returned.
func CreateVSHCommand(ctx context.Context, cid int, command string, args ...string) *testexec.Cmd {
	params := append([]string{"--cid=" + strconv.Itoa(cid), "--", command}, args...)
	cmd := testexec.CommandContext(ctx, "vsh", params...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}
