// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	terminaComponentName             = "cros-termina" // name of the Chrome component for the VM kernel and rootfs
	terminaComponentDownloadPath     = "/usr/local/cros-termina"
	terminaComponentLiveUrlFormat    = "https://storage.googleapis.com/termina-component-testing/%d/live"
	terminaComponentStagingUrlFormat = "https://storage.googleapis.com/termina-component-testing/%d/staging"
	terminaComponentUrlFormat        = "https://storage.googleapis.com/termina-component-testing/%d/%s/chromeos_%s-archive/files.zip"
	terminaMountDir                  = "/run/imageloader/cros-termina/99999.0.0"

	componentUpdaterName      = "org.chromium.ComponentUpdaterService"
	componentUpdaterPath      = dbus.ObjectPath("/org/chromium/ComponentUpdaterService")
	componentUpdaterInterface = "org.chromium.ComponentUpdaterService"

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

// CreateDefaultContainer prepares a VM and container with default settings and
// either the live or staging container versions.
func CreateDefaultContainer(ctx context.Context, user string, t ContainerType) (*Container, error) {
	concierge, err := NewConcierge(ctx, user)
	if err != nil {
		return nil, err
	}

	vmInstance := GetDefaultVM(concierge)

	if err := vmInstance.Start(ctx); err != nil {
		return nil, err
	}

	c, err := vmInstance.NewContainer(ctx, t)
	if err != nil {
		return nil, err
	}

	if err := c.StartAndWait(ctx); err != nil {
		return nil, err
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
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to mount component")
	}

	return nil
}

func mountComponentUpdater(ctx context.Context) error {
	_, updater, err := dbusutil.Connect(ctx, componentUpdaterName, dbus.ObjectPath(componentUpdaterPath))
	if err != nil {
		return err
	}

	var resp string
	testing.ContextLogf(ctx, "Mounting %q component", terminaComponentName)
	err = updater.CallWithContext(ctx, componentUpdaterInterface+".LoadComponent", 0, terminaComponentName).Store(&resp)
	if err != nil {
		return errors.Wrapf(err, "mounting %q component failed", terminaComponentName)
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

	milestone, err := getMilestone()
	if err != nil {
		return err
	}

	var url string
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
