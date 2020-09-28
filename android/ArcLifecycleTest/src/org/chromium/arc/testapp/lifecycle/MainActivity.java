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
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.widget.TextView;
import java.nio.ByteBuffer;
import java.nio.LongBuffer;
import java.util.ArrayList;
import java.util.Random;

public class MainActivity extends Activity {

  private static final String TAG = "ArcLifecycleTest";

  private static final String EXTRA_SIZE = "size";

  private static final long MB_BYTES = 1048576L;

  private String mActionAlloc;
  private String mActionDone;
  private TextView mTextView = null;
  private Handler mMainHandler = new Handler(Looper.getMainLooper());
  private Random mRandom = new Random();
  private Long mAllocatedSize = 0L;
  private Long mToAllocateSize = 0L;
  private ArrayList<ByteBuffer> mAllocated = new ArrayList<>();
  private Runnable mAllocateRunnable =
      new Runnable() {
        @Override
        public void run() {
          // Allocate 100 MB at a time.
          long allocSize = Math.min(mToAllocateSize, MB_BYTES * 100);
          allocate(allocSize);
          mToAllocateSize -= allocSize;
          if (mToAllocateSize > 0) {
            mMainHandler.post(mAllocateRunnable);
          } else {
            mTextView.setText(mAllocatedSize.toString());
          }
        }
      };
  private BroadcastReceiver mReceiver =
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          try {
            String action = intent.getAction();
            if (action.equals(mActionAlloc)) {
              boolean kickRunnable = (mToAllocateSize == 0);
              mToAllocateSize += intent.getLongExtra(EXTRA_SIZE, 1 * MB_BYTES);
              setResultData(Long.toString(mToAllocateSize + mAllocatedSize));
              if (kickRunnable) {
                mMainHandler.post(mAllocateRunnable);
                mTextView.setText("Allocating...");
              }
            } else if (action.equals(mActionDone)) {
              setResultData(Long.toString(mToAllocateSize));
            } else {
              throw new RuntimeException("Unrecognised intent action " + intent.getAction());
            }
            setResultCode(Activity.RESULT_OK);
          } catch (Exception e) {
            setResultCode(Activity.RESULT_CANCELED);
            setResultData(e.toString());
            Log.e(TAG, "Error in " + intent.getAction(), e);
          }
        }
      };

  private void allocate(long size) {
    while (size > 0) {
      long bufferSize = Math.min(size, MB_BYTES);
      allocateBuffer((int) bufferSize);
      size -= bufferSize;
    }
    mTextView.setText("Allocated: " + mAllocatedSize.toString());
  }

  private void allocateBuffer(int size) {
    ByteBuffer buffer = ByteBuffer.allocateDirect(size);
    LongBuffer longBuffer = buffer.asLongBuffer();
    for (int i = 0; i < longBuffer.capacity(); i++) {
      longBuffer.put(mRandom.nextLong());
    }
    mAllocated.add(buffer);
    mAllocatedSize += size;
  }

  private IntentFilter getFilter() {
    IntentFilter filter = new IntentFilter();
    filter.addAction(mActionAlloc);
    filter.addAction(mActionDone);
    return filter;
  }

  // Activity methods

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);

    setTitle(getPackageName());
    setContentView(R.layout.main_activity);
    mTextView = (TextView) findViewById(R.id.text);
    mTextView.setText("Waiting for ALLOC intent");

    // Dynamically create action strings because we change the package name when
    // building, so we can't do it statically.
    mActionAlloc = getPackageName() + ".ALLOC";
    mActionDone = getPackageName() + ".DONE";
    this.registerReceiver(mReceiver, getFilter());
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();

    mAllocated.clear();
    mAllocatedSize = 0l;
    this.unregisterReceiver(mReceiver);
  }
}
