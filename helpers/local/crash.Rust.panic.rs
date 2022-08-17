// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

use libchromeos::panic_handler::install_memfd_handler;

fn main() -> Result<(), ()> {
    install_memfd_handler();
    panic!("See you later, alligator!")
}
