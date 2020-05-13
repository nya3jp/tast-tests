/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowmanager;

import android.app.Activity;

/**
 * A {@link BaseActivity} subclass.
 * It is launched as the MAIN Intent. And also launched when a Unspecified and Non-Immersive
 * {@link Activity} is launched from the "Options for New Activity".
 */
public class MainActivity extends BaseActivity {}
