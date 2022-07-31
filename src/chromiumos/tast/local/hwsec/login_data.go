// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func runCmdOrFailWithOut(cmd *testexec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Return programs's output on failures. Avoid line breaks in error
		// messages, to keep the Tast logs readable.
		outFlat := strings.Replace(string(out), "\n", " ", -1)
		return errors.Wrap(err, outFlat)
	}
	return nil
}

func decompressData(ctx context.Context, src string) error {
	// Use the "tar" program as it takes care of recursive unpacking,
	// preserving ownership, permissions and SELinux attributes.
	cmd := testexec.CommandContext(ctx, "/bin/tar",
		"--extract",              // extract files from an archive
		"--gzip",                 // filter the archive through gunzip
		"--preserve-permissions", // extract file permissions
		"--same-owner",           // extract file ownership
		"--file",                 // read from the file specified in the next argument
		src)
	// Set the work directory for "tar" at "/", so that it unpacks files
	// at correct locations.
	cmd.Dir = "/"
	return runCmdOrFailWithOut(cmd)
}

func compressData(ctx context.Context, dst string, paths, ignorePaths []string) error {
	// Use the "tar" program as it takes care of recursive packing,
	// preserving ownership, permissions and SELinux attributes.
	args := append([]string{
		"--acls",    // save the ACLs to the archive
		"--create",  // create a new archive
		"--gzip",    // filter the archive through gzip
		"--selinux", // save the SELinux context to the archive
		"--xattrs",  // save the user/root xattrs to the archive
		"--file",    // write to the file specified in the next argument
		dst})
	for _, p := range ignorePaths {
		// Exclude the specified patterns from archiving.
		args = append(args, "--exclude", p)
	}
	// Specify the input paths to archive.
	args = append(args, paths...)
	cmd := testexec.CommandContext(ctx, "/bin/tar", args...)
	return runCmdOrFailWithOut(cmd)
}

// SaveLoginData creates the compressed file of login data:
// - /home/.shadow
// - /home/chronos
// - /mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip (if includeTpm is set to true).
func SaveLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string, includeTpm bool) error {
	if err := stopHwsecDaemons(ctx, daemonController, includeTpm); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController, includeTpm)

	paths := []string{
		"/home/.shadow",
		"/home/chronos",
	}
	if includeTpm {
		paths = append(paths, "/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip")
	}
	// Skip packing the "mount" directories, since the file systems it's
	// used for don't allow taking snapshots. E.g., ext4 fscrypt complains
	// "Required key not available" when trying to read encrypted files.
	ignorePaths := []string{
		"/home/.shadow/*/mount",
	}
	if err := compressData(ctx, archivePath, paths, ignorePaths); err != nil {
		return errors.Wrap(err, "failed to compress the cryptohome data")
	}
	return nil
}

// removeAllChildren deletes all files and folders from the specified directory.
func removeAllChildren(dirPath string) error {
	dir, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return errors.Wrap(err, "failed to read dir")
	}
	firstErr := error(nil)
	for _, f := range dir {
		fullPath := path.Join([]string{dirPath, f.Name()}...)
		if err := os.RemoveAll(fullPath); err != nil {
			// Continue even after seeing an error, to at least attempt
			// deleting other files.
			firstErr = errors.Wrapf(err, "failed to remove %s", f)
		}
	}
	return firstErr
}

// LoadLoginData loads the login data from compressed file.
func LoadLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string, includeTpm bool) error {
	if err := stopHwsecDaemons(ctx, daemonController, includeTpm); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController, includeTpm)

	// Remove the `/home/.shadow` first to prevent any unexpected file remaining.
	if err := os.RemoveAll("/home/.shadow"); err != nil {
		return errors.Wrap(err, "failed to remove old /home/.shadow data")
	}
	// Clean up `/home/chronos` as well (note that deleting this directory itself would fail).
	if err := removeAllChildren("/home/chronos"); err != nil {
		return errors.Wrap(err, "failed to remove old /home/chronos data")
	}

	if err := decompressData(ctx, archivePath); err != nil {
		return errors.Wrap(err, "failed to decompress the cryptohome data")
	}

	// Run `restorecon` to make sure SELinux attributes are correct after the decompression.
	if err := testexec.CommandContext(ctx, "restorecon", "-r", "/home/.shadow").Run(); err != nil {
		return errors.Wrap(err, "failed to restore selinux attributes")
	}
	return nil
}

func stopHwsecDaemons(ctx context.Context, daemonController *hwsec.DaemonController, includeTpm bool) error {
	if err := daemonController.TryStop(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to try to stop UI")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
	if includeTpm {
		if err := daemonController.TryStop(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
			return errors.Wrap(err, "failed to try to stop tpm2-simulator")
		}
	}
	return nil
}

func ensureHwsecDaemons(ctx context.Context, daemonController *hwsec.DaemonController, includeTpm bool) {
	if includeTpm {
		if err := daemonController.Ensure(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
			testing.ContextLog(ctx, "Failed to ensure tpm2-simulator: ", err)
		}
	}
	if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to ensure low-level TPM daemons: ", err)
	}
	if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to ensure high-level TPM daemons: ", err)
	}
	if err := daemonController.Ensure(ctx, hwsec.UIDaemon); err != nil {
		testing.ContextLog(ctx, "Failed to ensure UI: ", err)
	}
}
