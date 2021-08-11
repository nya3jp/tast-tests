/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.chromewebapk;

import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ResolveInfo;
import android.os.Bundle;
import android.net.Uri;

import java.util.List;
import java.util.ArrayList;

/**
 * Main activity for the ArcChromeWebApk test app.
 *
 * <p>Shows buttons to share content. If a single WebAPK is installed, will share the content to
 * that WebAPK. Otherwise, opens the system sharesheet.
 */
public final class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.content_view);

        findViewById(R.id.share_text_button).setOnClickListener(v -> {
            Intent textShareIntent = new Intent(Intent.ACTION_SEND);
            textShareIntent.setType("text/plain");
            textShareIntent.putExtra(Intent.EXTRA_SUBJECT, "Shared title");
            textShareIntent.putExtra(Intent.EXTRA_TEXT, "Shared text");

            startActivity(getWebApkTargetedIntent(textShareIntent));
        });

        findViewById(R.id.share_files_button).setOnClickListener(v -> {
            Intent fileShareIntent = new Intent(Intent.ACTION_SEND_MULTIPLE);

            ArrayList<Uri> uris = new ArrayList<>();
            uris.add(InMemoryContentProvider.getContentUri("file1.json"));
            uris.add(InMemoryContentProvider.getContentUri("file2.json"));

            fileShareIntent.setType("application/json");
            fileShareIntent.putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris);
            startActivity(getWebApkTargetedIntent(fileShareIntent));
        });
    }

    private Intent getWebApkTargetedIntent(Intent shareIntent) {
        PackageManager pm = getPackageManager();
        List<ResolveInfo> resolveInfos = pm.queryIntentActivities(shareIntent, 0);

        String packageName = null;

        for (ResolveInfo info : resolveInfos) {
            if (info.activityInfo.packageName.startsWith("org.chromium.webapk.")) {
                if (packageName != null) {
                    // There are multiple WebAPKs able to handle the intent, show a Chooser.
                    return Intent.createChooser(shareIntent, null);
                }
                packageName = info.activityInfo.packageName;
            }
        }

        if (packageName == null) {
            // There are no WebAPKs able to handle the intent, show a Chooser.
            return Intent.createChooser(shareIntent, null);
        }

        Intent targetedIntent = new Intent(shareIntent);
        targetedIntent.setPackage(packageName);
        return targetedIntent;
    }
}
