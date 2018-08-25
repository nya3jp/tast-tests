// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	cpb "chromiumos/system_api/vm_cicerone_proto" // protobufs for container management
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"golang.org/x/sys/unix"
)

const (
	terminaComponentName             = "cros-termina" // name of the Chrome component for the VM kernel and rootfs
	terminaComponentDownloadPath     = "/usr/local/cros-termina"
	terminaComponentLiveUrlFormat    = "https://storage.googleapis.com/termina-component-testing/%d/live"
	terminaComponentStagingUrlFormat = "https://storage.googleapis.com/termina-component-testing/%d/staging"
	terminaComponentUrlFormat        = "https://storage.googleapis.com/termina-component-testing/%d/%s/chromeos_%s-archive/files.zip"
	terminaMountDir                  = "/run/imageloader/cros-termina/99999.0.0"

	lsbReleasePath = "/etc/lsb-release"
	milestoneKey   = "CHROMEOS_RELEASE_CHROME_MILESTONE"
)

type ComponentType int

const (
	// ComponentUpdater indicates that the live component should be fetched from the component updater service.
	ComponentUpdater ComponentType = iota
	// LiveComponent indicates that the current live component should be fetched from the GS component testing bucket.
	LiveComponent
	// StagingComponent indicates that the current staging component should be fetched from the GS component testing bucket.
	StagingComponent
)

// NewDefaultContainer prepares a VM and container with default settings and
// either the live or staging container versions.
func CreateDefaultContainer(ctx context.Context, user string, t ContainerType) (*Container, error) {
	started, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "ContainerStarted",
	})
	// Always close the ContainerStarted watcher regardless of success.
	defer started.Close(ctx)

	concierge, err := NewConcierge(ctx, user)
	if err != nil {
		return nil, err
	}

	vm, err := concierge.StartTerminaVM(ctx)
	if err != nil {
		return nil, err
	}

	c, err := vm.NewContainer(ctx, t)
	if err != nil {
		return nil, err
	}

	if err = c.Start(ctx); err != nil {
		return nil, err
	}

	if err = c.SetUpUser(ctx); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Waiting for ContainerStarted D-Bus signal")
	sigResult := &cpb.ContainerStartedSignal{}
	for sigResult.VmName != c.VM.name &&
		sigResult.ContainerName != c.containerName &&
		sigResult.OwnerId != c.VM.Concierge.ownerID {
		select {
		case sig := <-started.Signals:
			if len(sig.Body) == 0 {
				return nil, errors.New("ContainerStarted signal lacked a body")
			}
			buf, ok := sig.Body[0].([]byte)
			if !ok {
				return nil, errors.New("ContainerStarted signal body is not a byte slice")
			}
			if err := proto.Unmarshal(buf, sigResult); err != nil {
				return nil, fmt.Errorf("failed unmarshaling ContainerStarted body: %v", err)
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("didn't get ContainerStarted D-Bus signal: %v", ctx.Err())
		}
	}

	return c, nil
}

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
			return "", nil
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
	url := fmt.Sprintf(terminaComponentUrlFormat, milestone, version, componentArch)
	testing.ContextLogf(ctx, "Downloading VM component version %s from: %s", version, url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("component download failed: %s", resp.Status)
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
	output, err := unzipCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to unzip: %s err %v", string(output), err)
	}
	return imagePath, nil
}

// mountComponent mounts a component image from the provided image path.
func mountComponent(ctx context.Context, image string) error {
	if err := os.MkdirAll(terminaMountDir, 0755); err != nil {
		return err
	}
	// Unmount any existing component.
	unix.Unmount(terminaMountDir, 0)

	// We could call losetup manually and use the mount syscall... or
	// we could let mount(8) do the work.
	mountCmd := testexec.CommandContext(ctx, "mount", image, "-o", "loop", terminaMountDir)
	output, err := mountCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount component: %s err %v", string(output), err)
	}

	return nil
}

func mountComponentUpdater(ctx context.Context) error {
	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	updater := bus.Object(dbusutil.ComponentUpdaterName, dbus.ObjectPath(dbusutil.ComponentUpdaterPath))

	var resp string
	testing.ContextLogf(ctx, "Mounting %q component", terminaComponentName)
	err = updater.CallWithContext(ctx, dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, terminaComponentName).Store(&resp)
	if err != nil {
		return fmt.Errorf("mounting %q component failed: %v", terminaComponentName, err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	// Ensure that the 99999.0.0 component isn't used.
	// Unmount any existing component and delete the 99999.0.0 directory.
	unix.Unmount(terminaMountDir, 0)
	return os.RemoveAll(terminaMountDir)
}

// SetUpComponent sets up the VM component according to the specified ComponentType.
func SetUpComponent(ctx context.Context, c ComponentType) error {
	if c == ComponentUpdater {
		return mountComponentUpdater(ctx)
	}

	var url string
	milestone, err := getMilestone()
	if err != nil {
		return err
	}

	switch c {
	case LiveComponent:
		url = fmt.Sprintf(terminaComponentLiveUrlFormat, milestone)
	case StagingComponent:
		url = fmt.Sprintf(terminaComponentStagingUrlFormat, milestone)
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("component symlink download failed: %s", resp.Status)
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

// dequote strips shell-style quotes from a string. It does not do any
// validation of the string, and only removes all instances of single and
// double-quote characters.
func dequote(s string) string {
	return strings.Replace(strings.Replace(s, "'", "", -1), "\"", "", -1)
}

// getMilestone returns the Chrome OS milestone for this build.
func getMilestone() (int, error) {
	f, err := os.Open(lsbReleasePath)
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), "=")
		if len(s) != 2 {
			return 0, errors.New("failed to parse lsb-release entry")
		}
		if s[0] == milestoneKey {
			val, err := strconv.Atoi(dequote(s[1]))
			if err != nil {
				return 0, fmt.Errorf("%q is not a valid milestone number: %v", s[1], err)
			}
			return val, nil
		}
	}
	return 0, errors.New("no milestone key in lsb-release file")
}
