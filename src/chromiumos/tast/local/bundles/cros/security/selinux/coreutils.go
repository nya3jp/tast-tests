// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"os"
	"sync"
)

// Please sync with platform2/sepolicy/sepolicy/file_contexts/coreutils,
// which is obtained by `equery-$BOARD files coreutils`, and then
// manually filtered.
var coreutilsFiles []string = []string{
	"/bin/basename",
	"/bin/cat",
	"/bin/chgrp",
	"/bin/chmod",
	"/bin/chown",
	"/bin/chroot",
	"/bin/cp",
	"/bin/cut",
	"/bin/date",
	"/bin/dd",
	"/bin/df",
	"/bin/dir",
	"/bin/dirname",
	"/bin/du",
	"/bin/echo",
	"/bin/env",
	"/bin/expr",
	"/bin/false",
	"/bin/head",
	"/bin/ln",
	"/bin/ls",
	"/bin/mkdir",
	"/bin/mkfifo",
	"/bin/mknod",
	"/bin/mktemp",
	"/bin/mv",
	"/bin/pwd",
	"/bin/readlink",
	"/bin/rm",
	"/bin/rmdir",
	"/bin/seq",
	"/bin/sleep",
	"/bin/sort",
	"/bin/stty",
	"/bin/sync",
	"/bin/tail",
	"/bin/touch",
	"/bin/tr",
	"/bin/true",
	"/bin/tty",
	"/bin/uname",
	"/bin/vdir",
	"/bin/wc",
	"/bin/yes",
	"/usr/bin/[",
	"/usr/bin/arch",
	"/usr/bin/base32",
	"/usr/bin/base64",
	"/usr/bin/basename",
	"/usr/bin/chcon",
	"/usr/bin/chroot",
	"/usr/bin/cksum",
	"/usr/bin/comm",
	"/usr/bin/coreutils",
	"/usr/bin/csplit",
	"/usr/bin/cut",
	"/usr/bin/dir",
	"/usr/bin/dircolors",
	"/usr/bin/dirname",
	"/usr/bin/du",
	"/usr/bin/env",
	"/usr/bin/expand",
	"/usr/bin/expr",
	"/usr/bin/factor",
	"/usr/bin/fmt",
	"/usr/bin/fold",
	"/usr/bin/head",
	"/usr/bin/hostid",
	"/usr/bin/id",
	"/usr/bin/install",
	"/usr/bin/join",
	"/usr/bin/link",
	"/usr/bin/logname",
	"/usr/bin/md5sum",
	"/usr/bin/mkfifo",
	"/usr/bin/mktemp",
	"/usr/bin/nice",
	"/usr/bin/nl",
	"/usr/bin/nohup",
	"/usr/bin/nproc",
	"/usr/bin/numfmt",
	"/usr/bin/od",
	"/usr/bin/paste",
	"/usr/bin/pathchk",
	"/usr/bin/pinky",
	"/usr/bin/pr",
	"/usr/bin/printenv",
	"/usr/bin/printf",
	"/usr/bin/ptx",
	"/usr/bin/readlink",
	"/usr/bin/realpath",
	"/usr/bin/runcon",
	"/usr/bin/seq",
	"/usr/bin/sha1sum",
	"/usr/bin/sha224sum",
	"/usr/bin/sha256sum",
	"/usr/bin/sha384sum",
	"/usr/bin/sha512sum",
	"/usr/bin/shred",
	"/usr/bin/shuf",
	"/usr/bin/sleep",
	"/usr/bin/sort",
	"/usr/bin/split",
	"/usr/bin/stat",
	"/usr/bin/stdbuf",
	"/usr/bin/sum",
	"/usr/bin/tac",
	"/usr/bin/tail",
	"/usr/bin/tee",
	"/usr/bin/test",
	"/usr/bin/timeout",
	"/usr/bin/touch",
	"/usr/bin/tr",
	"/usr/bin/truncate",
	"/usr/bin/tsort",
	"/usr/bin/tty",
	"/usr/bin/uname",
	"/usr/bin/unexpand",
	"/usr/bin/uniq",
	"/usr/bin/unlink",
	"/usr/bin/users",
	"/usr/bin/vdir",
	"/usr/bin/wc",
	"/usr/bin/who",
	"/usr/bin/whoami",
	"/usr/bin/yes",
}

var coreutilsOnce sync.Once          // initialize coreutilsSet
var coreutilsSet map[string]struct{} // keys are files belonging to coreutils.

// IsCoreutilsFile is a FileLabelCheckFilter that returns true if the given
// file belongs to the coreutils package.
func IsCoreutilsFile(p string, fi os.FileInfo) bool {
	coreutilsOnce.Do(func() {
		coreutilsSet = make(map[string]struct{})
		for _, f := range coreutilsFiles {
			coreutilsSet[f] = struct{}{}
		}
	})
	_, ok := coreutilsSet[p]
	return ok
}
