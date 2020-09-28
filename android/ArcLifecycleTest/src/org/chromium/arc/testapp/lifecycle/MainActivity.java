/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.lifecycle;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SharedMemory;
import android.widget.TextView;
import java.nio.ByteBuffer;
import java.nio.LongBuffer;
import java.util.ArrayList;
import java.util.Random;

public class MainActivity extends Activity {

  private static final String TAG = "ArcMemoryAllocatorTest";

  private static final String EXTRA_TO_ALLOCATE = "to_allocate";

  private static final long MB_BYTES = 1048576L;

  private TextView mTextView = null;
  private Handler mMainHandler = new Handler(Looper.getMainLooper());
  private Random mRandom = new Random();
  private Long mAllocatedSize = 0L;
  private Long mToAllocate = 0L;
  private ArrayList<ByteBuffer> mAllocated = new ArrayList<>();
  private Runnable mAllocateRunnable =
      new Runnable() {
        @Override
        public void run() {
          // Allocate 100 MB at a time.
          long allocSize = Math.min(mToAllocate, MB_BYTES * 100);
          if (allocSize <= 0) {
            return;
          }

          allocate(allocSize);
          mToAllocate -= allocSize;
          mMainHandler.post(mAllocateRunnable);
        }
      };

  private void allocateBuffer(int size) {
    ByteBuffer buffer = ByteBuffer.allocateDirect(size);
    LongBuffer longBuffer = buffer.asLongBuffer();
    for (int i = 0; i < longBuffer.capacity(); i++) {
      // TODO: compression ratio, I would prefer to have each page have some random and some
      // zeros
      longBuffer.put(mRandom.nextLong());
    }
    mAllocated.add(buffer);
    mAllocatedSize += size;
  }

  private void allocate(long size) {
    while (size > 0) {
      long bufferSize = Math.min(size, MB_BYTES);
      allocateBuffer((int) bufferSize);
      size -= bufferSize;
    }
    mTextView.setText("Allocated: " + mAllocatedSize.toString());
  }

  // Activity methods

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);

    setContentView(R.layout.main_activity);
    mTextView = (TextView) findViewById(R.id.text);
    Intent intent = getIntent();
    mToAllocate = intent.getLongExtra(EXTRA_TO_ALLOCATE, 0);
    mMainHandler.post(mAllocateRunnable);
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();

    // Give allocated memory up to the GC.
    for (ByteBuffer buffer : mAllocated) {
      SharedMemory.unmap(buffer);
    }
    mAllocated.clear();
    mAllocatedSize = 0l;
  }
}
