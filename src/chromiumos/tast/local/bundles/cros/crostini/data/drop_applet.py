# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a window which will receive a file drop.
# It prints the names of the files and closes.

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk

class DropWindow(Gtk.Window):
  def __init__(self):
    super().__init__(title="gtk3_drop_demo")
    self.connect("drag-motion", self.on_drag_motion)
    self.connect("drag-drop", self.on_drag_drop)
    self.connect("drag-data-received", self.on_drag_data_received)
    self.drag_dest_set(0, [], 0)

  def on_drag_motion(self, widgt, context, c, y, time):
    Gdk.drag_status(context, Gdk.DragAction.COPY, time)
    return True

  def on_drag_drop(self, widget, context, x, y, time):
    widget.drag_get_data(context, Gdk.Atom.intern("text/uri-list", True), time)

  def on_drag_data_received(self, widget, drag_context, x, y, data, info, time):
    print(data.get_uris(), end="")
    drag_context.finish(True, False, time)
    self.close()

window = DropWindow()
window.present()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
