// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// The directory where images will be downloaded/extracted to.
const (
	imageDir     = "/usr/local/cros-termina"
	imageFile    = "image.ext4"
	TerminaImage = "/usr/local/cros-termina/image.ext4"
)

// TerminaImageExists returns true if TerminaImage file exists and is readable.
func TerminaImageExists() bool {
	_, err := os.Stat(TerminaImage)
	return err == nil
}

// DeleteImages deletes all images downloaded or extracted for the test by the other functions in this file.
func DeleteImages() error {
	if err := os.RemoveAll(imageDir); err != nil {
		return errors.Wrap(err, "failed to remove image directory")
	}
	return nil
}

// ExtractTermina extracts the termina images from the artifact tarball.
func ExtractTermina(ctx context.Context, artifactPath string) (string, error) {
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return "", err
	}

	// Extract the zip. We expect an image.ext4 file in the output.
	if err := testexec.CommandContext(ctx, "unzip", "-u", artifactPath, imageFile, "-d", imageDir).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip")
	}

	return TerminaImage, nil
}

// DownloadStagingTermina downloads the current staging termina image from Google Storage.
func DownloadStagingTermina(ctx context.Context) (string, error) {
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to mkdir for container image")
	}

	milestone, err := getMilestone()
	if err != nil {
		return "", err
	}

	var componentArch = "arm32"
	if runtime.GOARCH == "amd64" {
		componentArch = "intel64"
	}

	// Download the symlink to the staging version.
	var link bytes.Buffer
	linkURL := fmt.Sprintf("https://storage.googleapis.com/termina-component-testing/%d/staging", milestone)
	if err := downloadTo(ctx, linkURL, &link); err != nil {
		return "", errors.Wrapf(err, "termina staging symlink download from %s failed", linkURL)
	}
	version := strings.TrimSpace(link.String())

	// Download the files.zip from the component GS bucket.
	url := fmt.Sprintf("https://storage.googleapis.com/termina-component-testing/%d/%s/chromeos_%s-archive/files.zip", milestone, version, componentArch)
	filesPath := filepath.Join(imageDir, "files.zip")
	if err := downloadToFile(ctx, url, filesPath); err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(filesPath); err != nil {
			testing.ContextLogf(ctx, "Ignoring error deleting uneeded archive file: %s", err)
		}
	}()

	if err := os.RemoveAll(TerminaImage); err != nil {
		return "", errors.Wrapf(err, "failed to delete old image.ext4 from %s", imageDir)
	}

	// Extract image.ext4 from the zip.
	if err := testexec.CommandContext(ctx, "unzip", filesPath, imageFile, "-d", imageDir).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to unzip image.ext4 from %s", filesPath)
	}

	return TerminaImage, nil
}

func downloadTo(ctx context.Context, url string, dest io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("download failed: %s", resp.Status)
	}
	if _, err := io.Copy(dest, resp.Body); err != nil {
		return err
	}
	return nil
}

func downloadToFile(ctx context.Context, url, downloadPath string) error {
	testing.ContextLogf(ctx, "Downloading %s to %s", url, downloadPath)
	dest, err := os.Create(downloadPath)
	if err != nil {
		return err
	}
	defer dest.Close()
	if err := downloadTo(ctx, url, dest); err != nil {
		os.Remove(downloadPath)
		return err
	}
	return nil
}
