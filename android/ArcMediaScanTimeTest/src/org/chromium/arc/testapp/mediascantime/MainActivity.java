/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.mediascantime;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.BroadcastReceiver;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.TextView;
import android.content.IntentFilter;
/**
 * Main activity for the ArcMediaScanTimeTest app.
 *
 * <p>Used by tast test to read file in FilesApp. File content is read and shown in TextView for
 * validation.
 */
public class MainActivity extends Activity {
    public static final String LOG_TAG = MainActivity.class.getSimpleName();

    private TextView mfileContent;
    private long mStartTime = 0;
    private long mFinishedTime = 0;
    private boolean mFirstScanFinished = false;
    private BroadcastReceiver mMediaScanListener =
        new BroadcastReceiver() {
          @Override
          public void onReceive(Context context, Intent intent) {
            if (!mFirstScanFinished) {
              if (mStartTime == 0){
                mStartTime = SystemClock.uptimeMillis();
              } else {
                mFinishedTime = SystemClock.uptimeMillis();
                mfileContent.setText(Long.toString(mFinishedTime - mStartTime));
                mFirstScanFinished = true;
              }
            }
          }
        };

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);
        mfileContent = findViewById(R.id.file_content);
        final IntentFilter filter = new IntentFilter();
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_STARTED);
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_FINISHED);
        filter.addDataScheme("file");
        // [EDITED] getContext().registerReceiver(mMediaScanListener,filter);
        registerReceiver(mMediaScanListener, filter);
    }
    @Override
    public void onDestroy() {
        unregisterReceiver(mMediaScanListener);
        super.onDestroy();
    }
}
