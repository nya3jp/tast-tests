/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.multidisplay;

import android.content.pm.ActivityInfo;
import android.content.res.Configuration;
import android.util.Log;
import java.util.ArrayList;

/**
 * Entry of configuration change event.
 */
public class ConfigChangeEvent {
  private static final String TAG = "ConfigChangeEvent";

  /**
   * Whether the configuration change is handled by app.
   *
   * true: if Activity#onConfigurationChanged is invoked.
   * false: if Activity is relaunched
   */
  public final boolean handled;

  /**
   * If density is changed
   */
  public final boolean density;

  /**
   * If fontScale is changed
   */
  public final boolean fontScale;

  /**
   * If orientation is changed
   */
  public final boolean orientation;

  /**
   * If screenLayout is changed
   */
  public final boolean screenLayout;

  /**
   * If screenSize is changed
   */
  public final boolean screenSize;

  /**
   * If smallestScreenSize is changed.
   */
  public final boolean smallestScreenSize;

  private ConfigChangeEvent(boolean handled, boolean density, boolean fontScale,
      boolean orientation, boolean screenLayout, boolean screenSize, boolean smallestScreenSize) {
    this.handled = handled;
    this.density = density;
    this.fontScale = fontScale;
    this.orientation = orientation;
    this.screenLayout = screenLayout;
    this.screenSize = screenSize;
    this.smallestScreenSize = smallestScreenSize;
  }

  /**
   * Creates ConfigChangeEvent which records onConfigurationChanged
   * @param old Old configuration
   * @param current New configuration
   * @return ConfigChangeEvent which records onConfigurationChanged
   */
  public static ConfigChangeEvent handled(Configuration old, Configuration current) {
    return forDiff(true, old, current);
  }

  public static ConfigChangeEvent relaunched(Configuration old, Configuration current) {
    return forDiff(false, old, current);
  }

  private static ConfigChangeEvent forDiff(
      boolean handled, Configuration old, Configuration current) {
    final int diff = old.diff(current);
    Log.d(TAG, String.format("handled=%b diff:%s\nold:%s\ncurrent:%s", handled,
        configurationDiffToString(diff), old, current));
    return new ConfigChangeEvent(
        handled,
        (diff & ActivityInfo.CONFIG_DENSITY) != 0,
        (diff & ActivityInfo.CONFIG_FONT_SCALE) != 0,
        (diff & ActivityInfo.CONFIG_ORIENTATION) != 0,
        (diff & ActivityInfo.CONFIG_SCREEN_LAYOUT) != 0,
        (diff & ActivityInfo.CONFIG_SCREEN_SIZE) != 0,
        (diff & ActivityInfo.CONFIG_SMALLEST_SCREEN_SIZE) != 0);
  }

  /**
   * Returns string representations for diff
   *
   * The function is copied from Configuration#configurationDiffToString as Tast APK cannot access
   * hidden framework methods.
   *
   * @param diff Return value of Configuration#diff
   * @return String representation of diff
   */
  public static String configurationDiffToString(int diff) {
    ArrayList<String> list = new ArrayList<>();
    if ((diff & ActivityInfo.CONFIG_MCC) != 0) {
      list.add("CONFIG_MCC");
    }
    if ((diff & ActivityInfo.CONFIG_MNC) != 0) {
      list.add("CONFIG_MNC");
    }
    if ((diff & ActivityInfo.CONFIG_LOCALE) != 0) {
      list.add("CONFIG_LOCALE");
    }
    if ((diff & ActivityInfo.CONFIG_TOUCHSCREEN) != 0) {
      list.add("CONFIG_TOUCHSCREEN");
    }
    if ((diff & ActivityInfo.CONFIG_KEYBOARD) != 0) {
      list.add("CONFIG_KEYBOARD");
    }
    if ((diff & ActivityInfo.CONFIG_KEYBOARD_HIDDEN) != 0) {
      list.add("CONFIG_KEYBOARD_HIDDEN");
    }
    if ((diff & ActivityInfo.CONFIG_NAVIGATION) != 0) {
      list.add("CONFIG_NAVIGATION");
    }
    if ((diff & ActivityInfo.CONFIG_ORIENTATION) != 0) {
      list.add("CONFIG_ORIENTATION");
    }
    if ((diff & ActivityInfo.CONFIG_SCREEN_LAYOUT) != 0) {
      list.add("CONFIG_SCREEN_LAYOUT");
    }
    if ((diff & ActivityInfo.CONFIG_COLOR_MODE) != 0) {
      list.add("CONFIG_COLOR_MODE");
    }
    if ((diff & ActivityInfo.CONFIG_UI_MODE) != 0) {
      list.add("CONFIG_UI_MODE");
    }
    if ((diff & ActivityInfo.CONFIG_SCREEN_SIZE) != 0) {
      list.add("CONFIG_SCREEN_SIZE");
    }
    if ((diff & ActivityInfo.CONFIG_SMALLEST_SCREEN_SIZE) != 0) {
      list.add("CONFIG_SMALLEST_SCREEN_SIZE");
    }
    if ((diff & ActivityInfo.CONFIG_DENSITY) != 0) {
      list.add("CONFIG_DENSITY");
    }
    if ((diff & ActivityInfo.CONFIG_LAYOUT_DIRECTION) != 0) {
      list.add("CONFIG_LAYOUT_DIRECTION");
    }
    if ((diff & ActivityInfo.CONFIG_FONT_SCALE) != 0) {
      list.add("CONFIG_FONT_SCALE");
    }
    // New config introduced at R.
    final int CONFIG_ASSETS_PATHS = 0x80000000;
    final int CONFIG_WINDOW_CONFIGURATION = 0x20000000;
    if ((diff & CONFIG_ASSETS_PATHS) != 0) {
      list.add("CONFIG_ASSETS_PATHS");
    }
    if ((diff & CONFIG_WINDOW_CONFIGURATION) != 0) {
      list.add("CONFIG_WINDOW_CONFIGURATION");
    }
    StringBuilder builder = new StringBuilder("{");
    for (int i = 0, n = list.size(); i < n; i++) {
      builder.append(list.get(i));
      if (i != n - 1) {
        builder.append(", ");
      }
    }
    builder.append("}");
    return builder.toString();
  }
}
