/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.lifecycle;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.Random;

public class MainActivity extends Activity {
  private static final String EXTRA_SIZE = "size";
  private static final String EXTRA_RATIO = "ratio";
  private static final String EXTRA_TOKEN = "token";

  private static final long MB_BYTES = 1024 * 1024;
  private static final int PAGE_BYTES = 4096;

  private TextView mAllocatedView;
  private Thread mAllocThread;

  // Referenced by mAllocThread, but values are const while thread is alive.
  private String mTag;

  // Guarded by synchronized(this).
  private boolean mQuit = false;
  private ArrayList<byte[]> mAllocated = new ArrayList<>();

  private void updateAllocated(final long allocated) {
    runOnUiThread(
        new Runnable() {
          @Override
          public void run() {
            mAllocatedView.setText(Long.toString(allocated));
          }
        });
  }

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);

    String packageName = getPackageName();
    mTag = packageName.substring(packageName.lastIndexOf('.') + 1);

    setTitle(mTag);
    setContentView(R.layout.main_activity);

    Intent intent = getIntent();
    final long allocateSize = intent.getLongExtra(EXTRA_SIZE, 0);
    float ratio = intent.getFloatExtra(EXTRA_RATIO, 1.0f);
    final int randEnd = (int) (ratio * PAGE_BYTES);
    String tokenNullable = intent.getStringExtra(EXTRA_TOKEN);
    final String token = tokenNullable != null ? tokenNullable : "";

    ((TextView) findViewById(R.id.size)).setText(Long.toString(allocateSize));
    ((TextView) findViewById(R.id.ratio)).setText(Float.toString(ratio));
    mAllocatedView = (TextView) findViewById(R.id.allocated);
    mAllocatedView.setText("0");

    mAllocThread =
        new Thread(
            new Runnable() {
              @Override
              public void run() {
                try {
                  Random random = new Random();
                  long allocated = 0;
                  long lastUIUpdate = 0;
                  Log.i(
                      mTag,
                      "allocation starting, requested size: "
                          + Long.toString(allocateSize)
                          + ", randEnd: "
                          + Integer.toString(randEnd)
                          + " "
                          + token);
                  while (allocated < allocateSize) {
                    byte[] buffer = new byte[(int) Math.min(allocateSize - allocated, MB_BYTES)];

                    for (int i = 0; i < buffer.length; i++) {
                      // Fill the first randEnd bytes of each page with random
                      // values, so that each page's compression ratio is
                      if ((i % PAGE_BYTES) < randEnd) {
                        buffer[i] = (byte) random.nextInt(256);
                      } else {
                        buffer[i] = 0;
                      }
                    }

                    allocated += buffer.length;
                    synchronized (MainActivity.this) {
                      mAllocated.add(buffer);
                      if (mQuit) {
                        break;
                      }
                    }

                    // Update the UI every 100ms.
                    long now = SystemClock.uptimeMillis();
                    if (now - lastUIUpdate > 100) {
                      lastUIUpdate = now;
                      updateAllocated(allocated);
                    }
                  }
                  updateAllocated(allocated);
                  Log.i(mTag, "allocation complete " + token);
                } catch (Exception e) {
                  Log.e(mTag, "allocation failed: " + e.toString());
                }
              }
            });
    mAllocThread.start();
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();

    synchronized (this) {
      mQuit = true;
    }
    try {
      mAllocThread.join();
    } catch (InterruptedException e) {
      Log.e(mTag, "failed to wait for allocation thread");
    }
    synchronized (this) {
      mAllocated.clear();
    }
  }
}
