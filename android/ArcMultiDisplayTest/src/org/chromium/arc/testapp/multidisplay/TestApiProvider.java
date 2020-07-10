/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.multidisplay;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.Bundle;
import android.os.CancellationSignal;
import android.os.Handler;
import android.os.Looper;
import android.util.ArrayMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.FutureTask;

/**
 * Content provider which provides information to Tast test.
 */
public class TestApiProvider extends ContentProvider {
  /**
   * Handler of main looper.
   */
  private final Handler mHandler;

  private static final String[] CONFIG_CHANGES_COLUMN_NAMES = {
      "activityId",
      "handled",
      "density",
      "fontScale",
      "orientation",
      "screenLayout",
      "screenSize",
      "smallestScreenSize"
  };

  private static final String CONFIG_CHANGES_PATH = "/configChanges";

  public TestApiProvider() {
    mHandler = new Handler(Looper.getMainLooper());
  }

  @Override
  public boolean onCreate() {
    return true;
  }

  @Override
  public Cursor query(Uri uri, String[] projection, String where, String[] args, String order) {
    throw new UnsupportedOperationException();
  }

  @Override
  public Cursor query(Uri uri, String[] projection, Bundle queryArgs,
      CancellationSignal cancellationSignal) {
    if (Looper.getMainLooper().isCurrentThread()) {
      throw new IllegalStateException("Should be invoked on a binder thread");
    }

    if (!CONFIG_CHANGES_PATH.equals(uri.getPath())) {
      return null;
    }

    final FutureTask<Cursor> futureCursor = new FutureTask<>(() -> {
      final ArrayMap<Integer, List<ConfigChangeEvent>> events =
          BaseActivity.getConfigChangeEvents();
      final MatrixCursor cursor = new MatrixCursor(CONFIG_CHANGES_COLUMN_NAMES);
      boolean completed = false;
      try {
        for (final Map.Entry<Integer, List<ConfigChangeEvent>> eventList : events.entrySet()) {
          for (final ConfigChangeEvent event : eventList.getValue()) {
            cursor.newRow().add(eventList.getKey()).add(event.handled).add(event.density)
                .add(event.fontScale).add(event.orientation).add(event.screenLayout)
                .add(event.screenSize).add(event.smallestScreenSize);
          }
        }
        completed = true;
        return cursor;
      } finally {
        if (!completed) {
          cursor.close();
        }
      }
    });

    if (!mHandler.post(futureCursor)) {
      throw new IllegalStateException("Main looper has already been ended");
    }

    try {
      return futureCursor.get();
    } catch (ExecutionException|InterruptedException e) {
      throw new RuntimeException(e);
    }
  }

  @Override
  public String getType(Uri uri) {
    return null;
  }

  @Override
  public Uri insert(Uri uri, ContentValues contentValues) {
    return null;
  }

  @Override
  public int delete(Uri uri, String s, String[] strings) {
    return 0;
  }

  @Override
  public int update(Uri uri, ContentValues contentValues, String s, String[] strings) {
    return 0;
  }
}
