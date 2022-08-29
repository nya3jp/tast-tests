/*
 * Copyright 2022 The ChromiumOS Authors.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowanimationjank;

import android.app.Instrumentation;
import android.app.UiAutomation;
import android.content.ComponentName;
import android.content.Intent;
import android.os.SystemClock;
// import android.support.test.uiautomator.By;
// import android.support.test.uiautomator.BySelector;
// import android.support.test.uiautomator.UiDevice;
// import android.support.test.uiautomator.UiObject2;
// import android.support.test.uiautomator.Until;

/**
 * Set of helpers to manipulate test activities.
 */
public class Utils {
    protected final static String PACKAGE = "android.windowanimationjank";
    protected final static String ELEMENT_LAYOUT_ACTIVITY = "ElementLayoutActivity";
    protected final static String ELEMENT_LAYOUT_CLASS = PACKAGE + "." + ELEMENT_LAYOUT_ACTIVITY;
    protected final static long WAIT_FOR_ACTIVITY_TIMEOUT = 10000;
    private final static long ROTATION_ANIMATION_TIME_FULL_SCREEN_MS = 1000;
    protected final static int ROTATION_MODE_NATURAL = 0;
    protected final static int ROTATION_MODE_LEFT = 1;
    protected final static int ROTATION_MODE_RIGHT = 2;

}