# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This app checks the clipboard every second, and exits itself if the clipboard
# data is "secret"

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk, GObject
import sys

def check():
  if Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD).wait_for_text() == 'secret':
    Gtk.main_quit()
    return False
  return True

window = Gtk.Window()
window.present()
window.connect('delete-event', Gtk.main_quit)
GObject.timeout_add(1000, check)
Gtk.main()
