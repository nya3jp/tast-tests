/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.lifecycle;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.TextView;
import java.util.ArrayList;
import java.util.Random;

public class MainActivity extends Activity {
  private static final String EXTRA_SIZE = "size";
  private static final String EXTRA_RATIO = "ratio";

  private static final long MB_BYTES = 1024 * 1024;
  private static final int PAGE_BYTES = 4096;

  private TextView mAllocatedView;
  private Thread mAllocThread;
  private BroadcastReceiver mReceiver;

  // Referenced by mAllocThread, but values are const while thread is alive.
  private String mTag;

  // Guarded by synchronized(this).
  private boolean mQuit = false;
  private boolean mDone = false;
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

    ((TextView) findViewById(R.id.size)).setText(Long.toString(allocateSize));
    ((TextView) findViewById(R.id.ratio)).setText(Float.toString(ratio));
    mAllocatedView = (TextView) findViewById(R.id.allocated);
    mAllocatedView.setText("0");

    final String actionDone = packageName + ".DONE";
    IntentFilter filter = new IntentFilter();
    filter.addAction(actionDone);
    mReceiver =
        new BroadcastReceiver() {
          @Override
          public void onReceive(Context context, Intent intent) {
            try {
              String action = intent.getAction();
              if (action.equals(actionDone)) {
                synchronized (MainActivity.this) {
                  setResultData(Boolean.toString(mDone));
                }
              } else {
                throw new RuntimeException("Unrecognised intent action " + intent.getAction());
              }
              setResultCode(Activity.RESULT_OK);
            } catch (Exception e) {
              setResultCode(Activity.RESULT_CANCELED);
              setResultData(e.toString());
              Log.e(mTag, "Error in " + intent.getAction(), e);
            }
          }
        };
    registerReceiver(mReceiver, filter);

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
                          + Integer.toString(randEnd));
                  while (allocated < allocateSize) {
                    byte[] buffer = new byte[(int) Math.min(allocateSize - allocated, MB_BYTES)];

                    for (int i = 0; i < buffer.length; i++) {
                      // Fill the first randEnd bytes of each page with random
                      // values, so that each page's compression ratio is the
                      // ratio passed on the launch intent.
                      if ((i % PAGE_BYTES) < randEnd) {
                        buffer[i] = (byte) random.nextInt(256);
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
                  synchronized (MainActivity.this) {
                    mDone = true;
                  }
                  updateAllocated(allocated);
                  Log.i(mTag, "allocation complete");
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

    this.unregisterReceiver(mReceiver);
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
