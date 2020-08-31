#!/bin/bash -e
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

failed=0
files=$(git diff-tree --no-commit-id --name-only -r "$1" -- \
    src/chromiumos/tast/local/bundles/cros/crostini/)
for file in ${files}; do
    if ! grep 'Pre:\s*crostini\.' "${file}" &>/dev/null; then
        # Doesn't look like it uses a Crostini precondition, ignore
        continue
    fi
    if ! grep 'defer crostini.RunCrostiniPostTest' "${file}"; then
        echo -n "${file}: Missing call to Crostini post-test hooks."
        echo "Try adding e.g.:"
        echo -n "  defer crostini.RunCrostiniPostTest("
        echo "ctx, s.PreValue().(crostini.PreData))"
        failed=1
    fi
done
exit $failed
