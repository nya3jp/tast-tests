# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a window, waits for a keypress, and
# copies the data given on the commandline to the clipboard.

import sys
import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk, GLib

class CopyWindow(Gtk.Window):
  def __init__(self, data):
    super().__init__(title="gtk3_copy_demo")

    self.clipboard = Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD)
    self.data = data
    self.connect("key_press_event", self.copy)

  def copy(self, *args, **kwargs):
    self.clipboard.set_text(self.data, -1)
    # Delay closing by 1 second to give sommelier a chance to copy the data
    GLib.timeout_add(1000, self.close, None)

  def close(self, *args, **kwargs):
    super().close()

window = CopyWindow(sys.argv[1])
window.present()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
