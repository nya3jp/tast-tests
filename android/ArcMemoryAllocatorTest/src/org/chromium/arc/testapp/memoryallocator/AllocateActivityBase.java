/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.memoryallocator;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Handler;
import android.os.Looper;
import android.os.SharedMemory;
import android.system.ErrnoException;
import android.util.Log;
import android.widget.TextView;
import java.nio.ByteBuffer;
import java.nio.LongBuffer;
import java.util.ArrayList;
import java.util.Random;

public class AllocateActivityBase extends Activity {
  private static final String TAG = "AllocateActivity";

  private static final String ACTION_ALLOC = "org.chromium.arc.testapp.memoryallocator.ALLOC";
  private static final String ACTION_DONE = "org.chromium.arc.testapp.memoryallocator.DONE";

  private static final String EXTRA_BYTES = "bytes";

  private static final long MB_BYTES = 1048576L;

  private TextView mTextView = null;

  // Members to allocate native memory.
  private Handler mMainHandler = new Handler(Looper.getMainLooper());
  private Random mRandom = new Random();
  private Long mAllocatedSize = 0l;
  private Long mToAllocate = 0l;
  private ArrayList<ByteBuffer> mAllocated = new ArrayList<>();
  private Runnable mAllocateRunnable =
      new Runnable() {
        @Override
        public void run() {
          // Allocate 100 MB at a time.
          long bufferSize = Math.min(mToAllocate, MB_BYTES * 100);
          if (bufferSize <= 0) {
            mTextView.setText("ID " + mId + " Allocated " + mAllocatedSize + " bytes");
            return;
          }

          allocateBuffer((int) bufferSize);
          mToAllocate -= bufferSize;
          mMainHandler.post(mAllocateRunnable);
        }
      };

  // Members to handle DONE intent.
  private Integer mId = 0;
  private String mAllocAction;
  private String mDoneAction;
  private BroadcastReceiver mReceiver =
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          Log.d(TAG, "BroadcastReceiver " + intent.getAction());
          if (intent.getAction().equals(mAllocAction)) {
            mToAllocate += intent.getLongExtra(EXTRA_BYTES, 0);

            // Remove any existing allocation tasks, so we don't end up with
            // more than one in the Handler.
            mMainHandler.removeCallbacks(mAllocateRunnable);
            mTextView.setText("ID " + mId + " Allocating...");
            mMainHandler.post(mAllocateRunnable);

            setResultData(Boolean.toString(true));
            setResultCode(Activity.RESULT_OK);
          } else if (intent.getAction().equals(mDoneAction)) {
            setResultData(Boolean.toString(mToAllocate <= 0l));
            setResultCode(Activity.RESULT_OK);
          } else {
            setResultCode(Activity.RESULT_CANCELED);
          }
        }
      };

  private IntentFilter getFilter() {
    IntentFilter filter = new IntentFilter();
    filter.addAction(mAllocAction);
    filter.addAction(mDoneAction);
    return filter;
  }

  private void allocateBuffer(int size) {
    try {
      SharedMemory sm = SharedMemory.create(null, size);
      ByteBuffer buffer = sm.mapReadWrite();
      LongBuffer longBuffer = buffer.asLongBuffer();
      for (int i = 0; i < longBuffer.capacity(); i++) {
        // TODO: compression ratio, I would prefer to have each page have some
        // random and some zeros to simulate compressibility.
        longBuffer.put(mRandom.nextLong());
      }
      mAllocated.add(buffer);
      mAllocatedSize += size;
    } catch (ErrnoException ex) {
      Log.e(TAG, "Error allocating", ex);
    }
  }

  // onCreateHelper should be called from onCreate in derived classes. This
  // starts allocation and registers the BroadcastReceiver for querying done.
  protected void onCreateHelper(int id) {
    mId = id;
    mAllocAction = ACTION_ALLOC + mId.toString();
    mDoneAction = ACTION_DONE + mId.toString();
    this.registerReceiver(mReceiver, getFilter());
    Log.d(TAG, "Registered BroadcastReceiver");

    setContentView(R.layout.main_activity);
    mTextView = findViewById(R.id.text);

    mTextView.setText("ID " + mId.toString() + " Waiting for intent");
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();

    this.unregisterReceiver(mReceiver);
    Log.d(TAG, "Unregistered BroadcastReceiver");

    // Give allocated memory up to the GC.
    for (ByteBuffer buffer : mAllocated) {
      SharedMemory.unmap(buffer);
    }
    mAllocated.clear();
    mAllocatedSize = 0l;
  }
}
