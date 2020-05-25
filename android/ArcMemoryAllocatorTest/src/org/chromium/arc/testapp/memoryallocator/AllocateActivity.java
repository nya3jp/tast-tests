/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

final class AllocateActivity extends Activity {
  private static final String TAG = "AllocateActivity";

  private TextView mTextView = null;

  private Random mRandom = new Random();
  private Long mAllocatedSize = 0l;
  private ArrayList<ByteBuffer> mAllocated = new ArrayList<>();

  protected void allocateBuffer(int size) {
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

  protected void allocate(long size) {
    while (size > 0) {
      long bufferSize = Math.min(size, MB_BYTES);
      allocateBuffer((int) bufferSize);
      size -= bufferSize;
    }
    mTextView.setText("Allocated: " + mAllocatedSize.toString());
  }

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);

    setContentView(R.layout.main_activity);
    mTextView = (TextView) findViewById(R.id.text);
    // TODO: allocate

    mTextView.setText("Allocating...");
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();
    // TODO: free memory
  }
}
