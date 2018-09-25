#!/usr/bin/env python3
# Copyright 2018 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Pre-upload check to ensure new tests are marked informational."""

import re
import subprocess
import sys


# Regexp matching test file names.
_TEST_FILE_RE = re.compile(
    r'^src/chromiumos/tast/[^/]+/bundles/[^/]+/[^/]+/[^/]+\.go$')


def _GetNewFiles(commit):
  """Returns file paths newly added by a commit.

  Args:
    commit: Git commit hash.

  Returns:
    A list of file paths relative to the git repository root.
  """
  out = subprocess.check_output(
      ['git', 'diff-tree', '--no-commit-id', '-r', '--name-only',
       '--diff-filter=A', commit])
  return out.decode('utf-8').splitlines()


def _GetContent(commit, path):
  """Returns the content of a file at a specified commit.

  Args:
    commit: Git commit hash.
    path: File path relative to the git repository root.

  Returns:
    File content decoded as UTF-8.
  """
  out = subprocess.check_output(['git', 'show', '%s:%s' % (commit, path)])
  return out.decode('utf-8')


def CheckCommit(commit):
  """Runs a check against a specified commit.

  This checks if all newly added test files contains "informational".

  Args:
    commit: Git commit hash.

  Returns:
    A list of offending file paths.
  """
  bad_files = []
  for path in _GetNewFiles(commit):
    if _TEST_FILE_RE.search(path):
      content = _GetContent(commit, path)
      if '"informational"' not in content:
        bad_files.append(path)
  return bad_files


def main(argv):
  _, commit = argv

  bad_files = CheckCommit(commit)

  if bad_files:
    print('Tests in following new files should be marked informational:')
    print()
    for p in bad_files:
      print('  %s' % p)
    print()
    print('See: go/tast-docs/writing_tests.md#code-location')


if __name__ == '__main__':
  main(sys.argv)
