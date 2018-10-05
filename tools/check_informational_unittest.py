#!/usr/bin/env python3
# Copyright 2018 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Unit tests for check_informational.py."""

import unittest

import check_informational


class CheckInformationalTest(unittest.TestCase):
  def testGoodCommit(self):
    self.assertEqual(
        check_informational.CheckCommit('b5e2050bde25ae51'),
        [])

  def testBadCommit(self):
    # This is an ancient change when we did not have concept of "informational".
    self.assertEqual(
        check_informational.CheckCommit('82a43e913c33dc2e'),
        ['src/chromiumos/tast/local/bundles/cros/platform/check_processes.go'])

  def testNoNewTestCommit(self):
    self.assertEqual(
        check_informational.CheckCommit('3936edfa34f44792'),
        [])

  def testPromotionCommit(self):
    # This is a change promoting a test to critical.
    self.assertEqual(
        check_informational.CheckCommit('4f8100ff43a611de'),
        [])


if __name__ == '__main__':
  unittest.main()
