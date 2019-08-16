# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a window and pastes from the clipboard.

import sys
import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk, GLib

class PasteWindow(Gtk.Window):
  def __init__(self):
    super().__init__(title="gtk3_paste_demo")

    self.clipboard = Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD)
    GLib.timeout_add(1000, self.paste, None)
    self.paste()

  def paste(self, *args):
    text = self.clipboard.wait_for_text()
    if text is not None:
      print(text, end='')
      self.close()


window = PasteWindow()
window.present()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
