// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	cpb "chromiumos/system_api/vm_cicerone_proto" // Protobufs for container management.
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"golang.org/x/sys/unix"
)

const (
	terminaComponentName         = "cros-termina" // The name of the Chrome component for the VM kernel and rootfs.
	terminaComponentDownloadPath = "/usr/local/cros-termina"
	terminaComponentLiveUrl      = "https://storage.googleapis.com/termina-component-testing/live"
	terminaComponentStagingUrl   = "https://storage.googleapis.com/termina-component-testing/staging"
	terminaComponentUrlFormat    = "https://storage.googleapis.com/termina-component-testing/%s/chromeos_%s-archive/files.zip"
	terminaMountDir              = "/run/imageloader/cros-termina/99999.0.0"
)

type ComponentType int

const (
	ComponentUpdater = iota // Get the live component from component updater.
	LiveComponent           // Get the live component from GS component testing bucket.
	StagingComponent        // Get the staging component from GS component testing bucket.
)

// NewDefaultContainer will prepare a VM and container with default settings and
// either the live or staging container versions.
func NewDefaultContainer(ctx context.Context, user string, t ContainerType) (*Concierge, *VM, *Container, error) {
	started, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "ContainerStarted",
	})
	// Always close the ContainerStarted watcher regardless of success.
	defer started.Close()

	concierge, err := NewConcierge(ctx, user)
	if err != nil {
		return nil, nil, nil, err
	}

	vm, err := concierge.StartTerminaVM(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	c, err := vm.NewContainer(ctx, t)
	if err != nil {
		return nil, nil, nil, err
	}

	if err = c.Start(ctx); err != nil {
		return nil, nil, nil, err
	}

	if err = c.SetUpUser(ctx); err != nil {
		return nil, nil, nil, err
	}

	testing.ContextLog(ctx, "Waiting for ContainerStarted D-Bus signal")
	sigResult := &cpb.ContainerStartedSignal{}
	for sigResult.VmName != c.vm.name &&
		sigResult.ContainerName != c.containerName &&
		sigResult.OwnerId != c.vm.ownerId {
		select {
		case sig := <-started.Signals:
			if len(sig.Body) == 0 {
				return nil, nil, nil, errors.New("ContainerStarted signal lacked a body")
			}
			buf, ok := sig.Body[0].([]byte)
			if !ok {
				return nil, nil, nil, errors.New("ContainerStarted signal body is not a byte slice")
			}
			if err := proto.Unmarshal(buf, sigResult); err != nil {
				return nil, nil, nil, fmt.Errorf("failed unmarshaling ContainerStarted body: %v", err)
			}
		case <-ctx.Done():
			return nil, nil, nil, fmt.Errorf("didn't get ContainerStarted D-Bus signal: %v", ctx.Err())
		}
	}

	return concierge, vm, c, nil
}

// downloadComponent will download a component with the given version string.
// Returns the path to the image that holds the component.
func downloadComponent(ctx context.Context, version string) (string, error) {
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
	url := fmt.Sprintf(terminaComponentUrlFormat, version, componentArch)
	testing.ContextLogf(ctx, "Downloading VM component version %s from: %s", version, url)
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("component download failed: %s", resp.Status)
	}
	filesPath := filepath.Join(componentDir, "files.zip")
	filesZip, err := os.Create(filesPath)
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

// mountComponent will mount a component image.
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
	err = updater.Call(dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, terminaComponentName).Store(&resp)
	if err != nil {
		return fmt.Errorf("mounting %q component failed: %v", terminaComponentName, err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	// Ensure that the 99999.0.0 component isn't used.
	// Unmount any existing component and delete the 99999.0.0 directory.
	unix.Unmount(terminaMountDir, 0)
	return os.RemoveAll(terminaMountDir)
}

func SetUpComponent(ctx context.Context, c ComponentType) error {
	var url string
	switch c {
	case ComponentUpdater:
		return mountComponentUpdater(ctx)
	case LiveComponent:
		url = terminaComponentLiveUrl
	case StagingComponent:
		url = terminaComponentStagingUrl
	}
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("component symlink download failed: %s", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(body))

	imagePath, err := downloadComponent(ctx, version)
	if err != nil {
		return err
	}

	return mountComponent(ctx, imagePath)
}
