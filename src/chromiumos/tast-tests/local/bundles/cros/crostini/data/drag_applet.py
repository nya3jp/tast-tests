# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a window which will emulate dragging file
# ./crostini.txt.

import gi
import os
import sys
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk

class DragWindow(Gtk.Window):
  def __init__(self):
    super().__init__(title="gtk3_drag_demo")
    self.connect("drag-data-get", self.on_drag_data_get)
    self.connect("drag-end", self.on_drag_end)
    self.drag_source_set(Gdk.ModifierType.BUTTON1_MASK, [], Gdk.DragAction.COPY)
    self.drag_source_add_uri_targets()

  def on_drag_data_get(self, widget, drag_context, data, info, time):
    data.set_uris(["file://" + os.path.abspath(sys.argv[1])])

  def on_drag_end(self, drag_context, data):
    self.close()

window = DragWindow()
window.show_all()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
