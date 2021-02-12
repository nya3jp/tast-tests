/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.downloadmanager;

import android.app.Activity;
import android.app.DownloadManager;
import android.app.DownloadManager.Request;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.net.Uri;
import android.os.Bundle;
import android.widget.TextView;

import java.io.File;

public class MainActivity extends Activity {
    private static final String KEY_SOURCE_URL = "source_url";
    private static final String KEY_TARGET_PATH = "target_path";

    private static final String STATUS_STARTED = "Started";
    private static final String STATUS_FINISHED = "Finished";

    private final Object mLock = new Object();

    private DownloadManager mDownloadManager;
    private BroadcastReceiver mReceiver;

    // TODO(b/180082486): Stop commenting out the @GuardedBy annotation once we import or define it.
    // @GuardedBy("mLock")
    private long mDownloadManagerJobId = -1;

    private TextView mSource;
    private TextView mTarget;
    private TextView mStatus;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        final Intent intent = getIntent();
        final String sourceUrl = intent.getStringExtra(KEY_SOURCE_URL);
        final String targetPath = intent.getStringExtra(KEY_TARGET_PATH);

        setContentView(R.layout.main_activity);

        mSource = findViewById(R.id.source);
        mTarget = findViewById(R.id.target);
        mStatus = findViewById(R.id.status);

        mSource.setText(sourceUrl);
        mTarget.setText(targetPath);
        mStatus.setText(STATUS_STARTED);

        mDownloadManager = getSystemService(DownloadManager.class);

        mReceiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context context, Intent intent) {
                synchronized (mLock) {
                    final long id = intent.getLongExtra(DownloadManager.EXTRA_DOWNLOAD_ID,
                                                        mDownloadManagerJobId + 1 /*defaultValue*/);
                    if (id == mDownloadManagerJobId) {
                        mStatus.setText(STATUS_FINISHED);
                    }
                }
            }
        };
        IntentFilter intentFilter = new IntentFilter(DownloadManager.ACTION_DOWNLOAD_COMPLETE);
        registerReceiver(mReceiver, intentFilter);

        Request request = new Request(Uri.parse(sourceUrl));
        request.setDestinationUri(Uri.fromFile(new File(targetPath)));

        synchronized (mLock) {
            mDownloadManagerJobId = mDownloadManager.enqueue(request);
        }
    }

    @Override
    public void onDestroy() {
        unregisterReceiver(mReceiver);
        super.onDestroy();
    }
}
