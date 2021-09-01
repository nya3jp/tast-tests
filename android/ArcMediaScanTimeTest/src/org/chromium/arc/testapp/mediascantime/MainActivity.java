/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.mediascantime;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.net.Uri;
import android.os.Bundle;
import android.os.SystemClock;
import android.widget.TextView;

/**
 * Main activity for the ArcMediaScanTimeTest app.
 *
 * <p>Used by tast test to read file in FilesApp. File content is read and shown in TextView for
 * validation.
 */

public class MainActivity extends Activity {

    private TextView mMediaScanTime;
    private Uri mTargetBroadcastUri;
    private long mStartTime = 0;
    private boolean mFirstScanFinished = false;
    private BroadcastReceiver mMediaScanListener =
        new BroadcastReceiver() {
          @Override
          public void onReceive(Context context, Intent intent) {
            if (!mTargetBroadcastUri.equals(intent.getData())) {
              return;
            }
            if (!mFirstScanFinished) {
              if (mStartTime == 0 && intent.getAction() == Intent.ACTION_MEDIA_SCANNER_STARTED){
                mStartTime = SystemClock.uptimeMillis();
              } else if (intent.getAction() == Intent.ACTION_MEDIA_SCANNER_FINISHED){
                long finishedTime = SystemClock.uptimeMillis();
                mMediaScanTime.setText(Long.toString(finishedTime - mStartTime));
                mFirstScanFinished = true;
              }
            }
          }
        };

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);
        mTargetBroadcastUri = getIntent().getData();
        mMediaScanTime = findViewById(R.id.media_scan_time);
        final IntentFilter filter = new IntentFilter();
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_STARTED);
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_FINISHED);
        filter.addDataScheme("file");
        registerReceiver(mMediaScanListener, filter);
    }

    @Override
    public void onDestroy() {
        unregisterReceiver(mMediaScanListener);
        super.onDestroy();
    }
}
