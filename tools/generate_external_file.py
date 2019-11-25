#!/usr/bin/env python3
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""
Used to generate an external link format JSON file for the given tast data file.

In order to use provide the name of the data file that you want to upload as
well as the name of the test directory that the file is used for.

Usage:
  ./generate_external_file.py <data_file> <test_directory> [--upload]

  data_file: The name of the data_file used to produce the external link file.
  test_directory: The name of the test directory that |data_file| is used in.
                  For example if you are adding a data file for the test
                  "audio.Microphone" then you would pass "audio".
  upload: Whether to upload |data_file| to Google Cloud Storage.

Example:

  ./generate_external_file.py test_data.mp3 audio

Will produce a file called 'test_data.mp3.external' in the external link format
in the current directory.

If the '--upload' option is provided then the given data file will be uploaded
to the following path in Google Cloud Storage:

  //chromiumos-test-assets-public/tast/cros/<test_dir>/<data_file>.external
"""

import argparse
import hashlib
import json
import os
import subprocess
from datetime import datetime

_GCP_PREFIX = 'chromiumos-test-assets-public/tast/cros'


def _parse_args():
  parser = argparse.ArgumentParser()
  parser.add_argument('data_file', help='name of the data file')
  parser.add_argument(
      'test_dir',
      help='name of the associated test used to fill the url field')
  parser.add_argument(
      '--upload',
      help='upload data file to Google Cloud Storage',
      action='store_true')
  return parser.parse_args()


def _get_sha256_digest(path):
  sha256 = hashlib.sha256()
  with open(path, 'rb') as infile:
    while True:
      buf = infile.read(1024)
      if not buf:
        break
      sha256.update(buf)
  return sha256.hexdigest()


def main():
  args = _parse_args()

  timestamp = datetime.now().strftime('%Y%m%d-%H%M%S')
  url = 'gs://{prefix}/{test_dir}/{data_file}_{timestamp}'.format(
      prefix=_GCP_PREFIX,
      test_dir=args.test_dir,
      data_file=args.data_file,
      timestamp=timestamp)

  if not os.path.exists(args.data_file):
    print('No such file:', args.data_file)
    return

  size = os.path.getsize(args.data_file)
  digest = _get_sha256_digest(args.data_file)

  link = {'url': url, 'size': size, 'sha256sum': digest}

  # Write out the the JSON file in the external link format.
  external_file = args.data_file + '.external'

  # Warn the user if the file already exists.
  if os.path.exists(external_file):
    ans = input(
        'File {0} already exists. Overwrite it? Y/N '.format(external_file))
    if ans.lower() not in ['y', 'yes']:
      print('Exiting')
      return

  with open(external_file, 'w') as outfile:
    json.dump(link, outfile, sort_keys=True, indent=2)
    outfile.write('\n')

  if args.upload:
    try:
      print('Uploading file...')
      subprocess.check_call(['gsutil', 'cp', '-n', args.data_file, url])
    except subprocess.CalledProcessError as e:
      print('Failed to upload file')


if __name__ == '__main__':
  main()
