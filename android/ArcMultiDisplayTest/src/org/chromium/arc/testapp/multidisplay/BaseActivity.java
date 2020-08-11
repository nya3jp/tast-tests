/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.multidisplay;

import android.app.Activity;
import android.content.res.Configuration;
import android.os.Bundle;
import android.util.ArrayMap;
import android.util.Log;
import java.util.ArrayList;
import java.util.List;

public class BaseActivity extends Activity {

  private static final String TAG = "BaseActivity";

  private static final String KEY_CONFIGURATION = "configuration";
  private static final String KEY_ACTIVITIY_ID = "activityId";

  /**
   * Map between activityId and config events. Must access only on the main thread.
   */
  private static final ArrayMap<Integer, List<ConfigChangeEvent>> sConfigEvents = new ArrayMap<>();

  /**
   * Next activity ID. Must access on the main thread.
   */
  private static int sNextActivityId = 0;

  /**
   * Last notified configuration.
   */
  private final Configuration mConfiguration = new Configuration();

  /**
   * Activity ID.
   */
  private int mActivityId = -1;

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    if (savedInstanceState != null && savedInstanceState.containsKey(KEY_ACTIVITIY_ID)) {
      mActivityId = savedInstanceState.getInt(KEY_ACTIVITIY_ID);
    } else {
      mActivityId = sNextActivityId++;
    }

    mConfiguration.setTo(getResources().getConfiguration());
    if (savedInstanceState != null && savedInstanceState.containsKey(KEY_CONFIGURATION)) {
      Log.d(TAG, String.format("relaunched %d %s", mActivityId, mConfiguration));
      sConfigEvents.computeIfAbsent(mActivityId, key -> new ArrayList<>()).add(ConfigChangeEvent
          .relaunched(savedInstanceState.getParcelable(KEY_CONFIGURATION), mConfiguration));
    }

    Log.d(TAG, String.format("onCreate %d %s", mActivityId, mConfiguration));
    super.onCreate(savedInstanceState);
  }

  @Override
  public void onConfigurationChanged(Configuration newConfig) {
    Log.d(TAG, String.format("onConfigurationChanged %d %s", mActivityId, newConfig));
    super.onConfigurationChanged(newConfig);
    sConfigEvents
        .computeIfAbsent(mActivityId, key -> new ArrayList<>())
        .add(ConfigChangeEvent.handled(mConfiguration, newConfig));
    mConfiguration.setTo(newConfig);
  }

  @Override
  protected void onDestroy() {
    Log.d(TAG, "onDestroy " + mActivityId);
    super.onDestroy();
  }

  @Override
  protected void onSaveInstanceState(Bundle outState) {
    Log.d(TAG, "onSaveInstanceState " + mActivityId);
    super.onSaveInstanceState(outState);
    outState.putParcelable(KEY_CONFIGURATION, mConfiguration);
    outState.putInt(KEY_ACTIVITIY_ID, mActivityId);
  }

  /**
   * Returns config change events on each running Activity. Must be invoked on the main thread.
   *
   * @return config change events on each running Activity.
   */
  public static ArrayMap<Integer, List<ConfigChangeEvent>> getConfigChangeEvents() {
    return sConfigEvents;
  }
}
