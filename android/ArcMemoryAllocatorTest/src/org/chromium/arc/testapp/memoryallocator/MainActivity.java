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
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;
import java.nio.ByteBuffer;
import java.nio.LongBuffer;
import java.util.ArrayList;
import java.util.Random;

public class MainActivity extends Activity {

  private static final String TAG = "ArcMemoryAllocatorTest";

  private static final String ACTION_ALLOC = "org.chromium.arc.testapp.memoryallocator.ALLOC";

  private static final String EXTRA_SIZE = "size";

  private static final long MB_BYTES = 1048576L;

  private static IntentFilter getFilter() {
    IntentFilter filter = new IntentFilter();
    filter.addAction(ACTION_ALLOC);
    return filter;
  }

  private TextView mTextView = null;
  private Random mRandom = new Random();
  private Long mAllocatedSize = 0l;
  private ArrayList<ByteBuffer> mAllocated = new ArrayList<>();

  private BroadcastReceiver mReceiver =
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          try {
            switch (intent.getAction()) {
              case ACTION_ALLOC:
                // (intent.getLongExtra(EXTRA_SIZE, MB_BYTES));
                break;
              default:
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
    this.registerReceiver(mReceiver, getFilter());
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();
    this.unregisterReceiver(mReceiver);
  }
}
