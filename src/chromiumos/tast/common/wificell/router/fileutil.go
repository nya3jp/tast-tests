// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// PutFiles copies files on the local machine to the host. The files param
// describes a mapping from a local file path to a remote file path.
// For example, the call:
//
//	PutFiles(ctx, conn, map[string]string{"/src/from": "/dst/to"})
//
// will copy the local file or directory /src/from to /dst/to on the remote host.
// Local file paths can be absolute or relative. Remote file paths must be absolute.
// bytes is the amount of data sent over the wire. Local symbolic links are
// evaluated. All directories are created with the default permissions on the
// host.
//
// Unlike linuxssh.PutFiles, tar is not used to transfer the files. This is
// because routers have varied support for tar. Instead, each file is read and
// written directly with linuxssh.WriteFile using the same permissions as the
// source file. Also unlike linuxssh.PutFiles, no file compression is used and
// all files are always written.
func PutFiles(ctx context.Context, host *ssh.Conn, files map[string]string) (int64, error) {
	var bytesTransferred int64

	// Validate file paths and convert relative src paths to absolute
	absoluteFiles := make(map[string]string)
	srcFilesAndDirsQueue := make([]string, 0)
	for src, dst := range files {
		// Ensure src is absolute
		p, err := filepath.Abs(src)
		if err != nil {
			return 0, errors.Wrapf(err, "source path %q could not be resolved", src)
		}
		src = p
		// Fully evaluate symbolic links
		p, err = filepath.EvalSymlinks(src)
		if err != nil {
			return 0, errors.Wrapf(err, "source path %q could not be resolved", src)
		}
		src = p
		// Require destination path to be absolute
		dst = filepath.Clean(dst)
		if !filepath.IsAbs(dst) {
			return 0, errors.Errorf("destination path %q should be absolute", dst)
		}
		srcFilesAndDirsQueue = append(srcFilesAndDirsQueue, src)
		absoluteFiles[src] = dst
	}

	// Collect all files, walking directories as needed
	srcFilesQueue := make([]string, 0)
	srcFilePerms := map[string]os.FileMode{}
	dstDirSet := map[string]struct{}{}
	for len(srcFilesAndDirsQueue) > 0 {
		// Consume next in queue
		src := srcFilesAndDirsQueue[0]
		srcFilesAndDirsQueue = srcFilesAndDirsQueue[1:]

		// Handle path based on type of file
		srcFileInfo, err := os.Stat(src)
		if err != nil {
			return bytesTransferred, errors.Wrapf(err, "failed to stat source path %q", src)
		}
		if srcFileInfo.IsDir() {
			// Add files in dir to queue, skipping any already accounted for
			dirFiles, err := ioutil.ReadDir(src)
			if err != nil {
				return bytesTransferred, errors.Wrapf(err, "failed to read contents of source directory %q", src)
			}
			dirDst := absoluteFiles[src]
			for _, dirFileInfo := range dirFiles {
				dirFile := filepath.Join(dirDst, filepath.Base(dirFileInfo.Name()))
				dirFile, err = filepath.EvalSymlinks(dirFile)
				if err != nil {
					return bytesTransferred, errors.Wrapf(err, "source path %q in resolved dir %q could not be resolved", dirFileInfo.Name(), src)
				}
				if _, ok := absoluteFiles[dirFile]; !ok {
					absoluteFiles[dirFile] = dirFile
					srcFilesAndDirsQueue = append(srcFilesAndDirsQueue, dirFile)
				}
			}
		} else {
			// Collect dir and queue for file copy
			dstDirSet[filepath.Dir(absoluteFiles[src])] = struct{}{}
			srcFilesQueue = append(srcFilesQueue, src)
			srcFilePerms[src] = srcFileInfo.Mode().Perm()
		}
	}

	testing.ContextLogf(ctx, "Copying %d files to remote host", len(srcFilesQueue))

	// Make any needed directories on host
	dstDirs := make([]string, 0)
	for dstDir := range dstDirSet {
		dstDirs = append(dstDirs, dstDir)
	}
	if err := MakeDirs(ctx, host, dstDirs...); err != nil {
		return bytesTransferred, errors.Wrap(err, "failed to create destination directories")
	}

	// Put each file on host
	for _, src := range srcFilesQueue {
		dst := absoluteFiles[src]
		var data []byte
		var err error

		testing.ContextLogf(ctx, "Copying local file %q to remote file %q", src, dst)

		// Read local file contents
		if data, err = ioutil.ReadFile(src); err != nil {
			return bytesTransferred, errors.Wrapf(err, "failed to read source file %q", src)
		}

		// Write file contents on host
		if err := linuxssh.WriteFile(ctx, host, dst, data, srcFilePerms[src]); err != nil {
			return bytesTransferred, errors.Wrapf(err, "failed to write destination file %q", dst)
		}
		// Keep a running log of transferred bytes
		bytesTransferred += int64(len(data))
	}
	return bytesTransferred, nil
}

// MakeDirs ensures directories on the remote host exist matching the absolute
// paths in dirs, creating any missing directories in each path.
//
// Directories are created using "mkdir -p <path>". Since the "-p" flag creates
// any missing parent directories in the path as well, if one path in dirs is
// would be a parent of another path in dirs the parent path is not explicitly
// created with mkdir. All directories are created using default permissions
// on the host.
func MakeDirs(ctx context.Context, host *ssh.Conn, dirs ...string) error {
	// Validate paths are absolute before making any changes on host and clean paths
	parentDirSet := map[string]struct{}{}
	for i, dir := range dirs {
		if !filepath.IsAbs(dir) {
			return errors.Errorf("destination directory path %q should be absolute", dir)
		}
		dirs[i] = filepath.Clean(dirs[i])
		parentDirSet[filepath.Dir(dirs[i])] = struct{}{}
	}
	// Make dirs on host
	for _, dir := range dirs {
		if _, isParentOfAnotherDir := parentDirSet[dir]; !isParentOfAnotherDir {
			if err := host.CommandContext(ctx, "mkdir", "-p", dir).Run(); err != nil {
				return errors.Wrapf(err, "failed to make directory %q on host", dir)
			}
		}
	}
	return nil
}
