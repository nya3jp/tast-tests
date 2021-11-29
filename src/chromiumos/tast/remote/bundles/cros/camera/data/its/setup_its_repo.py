#!/usr/bin/env python3
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import subprocess
import argparse
import sys
from pathlib import Path


def parse_args(args):
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=
        'Script for setup ITS repo with python3 patch from CTS verifier bunle.',
        epilog='''
Example:
  $ wget https://dl.google.com/dl/android/cts/android-cts-verifier-9.0_r15-linux_x86-x86.zip
  $ python3 %(prog)s android-cts-verifier-9.0_r15-linux_x86-x86.zip
''')
    parser.add_argument(
        'bundle_path',
        help='ITS bundle download from: '
        'https://source.android.com/compatibility/cts/downloads.',
    )
    parser.add_argument(
        '--patch_path',
        required=False,
        help='Path to CTS verifier patch.',
    )
    parser.add_argument(
        '--output',
        default=".",
        help='Directory to setup ITS repo.',
    )
    return parser.parse_args(args)

def main(args):
    args = parse_args(args)
    bundle_path = Path(args.bundle_path)
    output = Path(args.output)
    if args.patch_path:
        patch_path = Path(args.patch_path)
    else:
        patch_path = Path(__file__).parent/'its.patch'

    # Prepare ITS repo.
    subprocess.check_call(['unzip', '-d', args.output, args.bundle_path])
    its_root = (output/'android-cts-verifier'/'CameraITS').resolve()
    subprocess.check_call(['git', 'init'], cwd=str(its_root))

    # Create and tag the base source commit.
    python_files = its_root.glob('**/*.py')
    cmd = [
        'git', 'add',
        *map(lambda p: str(p.relative_to(its_root)), python_files)
    ]
    subprocess.check_call(cmd, cwd=str(its_root))
    base_msg = 'Base ITS source'
    fake_author = 'testuser <testuser@gmail.com>'
    subprocess.check_call(
        ['git', 'commit', '--author', fake_author, '-m', base_msg],
        cwd=str(its_root))
    subprocess.check_call(['git', 'tag', '-a', 'base', '-m', base_msg],
                          cwd=str(its_root))

    # Apply python3 patch.
    subprocess.check_call(
        ['git', 'apply', str(patch_path.resolve())], cwd=str(its_root))


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
