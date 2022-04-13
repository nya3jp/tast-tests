/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputoverlay;

import android.content.Context;
import android.view.InputEvent;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.TextView;

import java.util.List;

class EventListAdapter extends BaseAdapter {
  private Context mContext;
  private List<ReceivedEvent> mList;

  public EventListAdapter(Context context, List<ReceivedEvent> list) {
    mContext = context;
    mList = list;
  }

  @Override
  public View getView(int i, View convertView, ViewGroup viewGroup) {
    View view = convertView;
    if (view == null) {
      view = LayoutInflater.from(mContext).inflate(R.layout.event_list_item, null);
    }
    final ReceivedEvent item = getItem(i);
    ((TextView) view.findViewById(R.id.m_item_action)).setText(item.action);
    ((TextView) view.findViewById(R.id.m_item_code)).setText(item.code);
    ((TextView) view.findViewById(R.id.m_item_action_button)).setText(item.actionButton);
    ((TextView) view.findViewById(R.id.m_item_source)).setText(item.source);
    ((TextView) view.findViewById(R.id.m_item_receive_time)).setText(item.receiveTimeNs.toString());
    return view;
  }

  @Override
  public int getCount() {
    return mList.size();
  }

  @Override
  public ReceivedEvent getItem(int i) {
    return mList.get(i);
  }

  @Override
  public long getItemId(int i) {
    return i;
  }
}
