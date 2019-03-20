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
	terminaComponentName             = "cros-termina" // name of the Chrome component for the VM kernel and rootfs
	terminaComponentDownloadPath     = "/usr/local/cros-termina"
	terminaComponentLiveURLFormat    = "https://storage.googleapis.com/termina-component-testing/%d/live"
	terminaComponentStagingURLFormat = "https://storage.googleapis.com/termina-component-testing/%d/staging"
	terminaComponentURLFormat        = "https://storage.googleapis.com/termina-component-testing/%d/%s/chromeos_%s-archive/files.zip"
	terminaMountDir                  = "/run/imageloader/cros-termina/99999.0.0"

	lsbReleasePath = "/etc/lsb-release"
	milestoneKey   = "CHROMEOS_RELEASE_CHROME_MILESTONE"
)

// ComponentType represents the VM component type.
type ComponentType int

const (
	// ComponentUpdater indicates that the live component should be fetched from the component updater service.
	ComponentUpdater ComponentType = iota
	// LiveComponent indicates that the current live component should be fetched from the GS component testing bucket.
	LiveComponent
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
	updater, err := compupdater.New(ctx)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Mounting %q component", terminaComponentName)
	resp, err := updater.LoadComponent(ctx, terminaComponentName, compupdater.Mount)
	if err != nil {
		return errors.Wrapf(err, "mounting %q component failed", terminaComponentName)
	}
	// Empty return path (when using compupdater.Mount option) indicates
	// component updater fails to install the given component.
	if resp == "" {
		return errors.Errorf("Component %q installation failed", terminaComponentName)
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
		url = fmt.Sprintf(terminaComponentLiveURLFormat, milestone)
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
	if err := unix.Unmount(terminaMountDir, 0); err != nil {
		testing.ContextLog(ctx, "Failed to unmount component: ", err)
	}

	if err := os.Remove(terminaMountDir); err != nil {
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

// parseIPv4 returns the first IPv4 address found in a space separated list of IPs.
func findIPv4(ips string) (string, error) {
	for _, v := range strings.Fields(ips) {
		ip := net.ParseIP(v)
		if ip != nil && ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", errors.Errorf("could not find IPv4 address in %q", ips)
}
