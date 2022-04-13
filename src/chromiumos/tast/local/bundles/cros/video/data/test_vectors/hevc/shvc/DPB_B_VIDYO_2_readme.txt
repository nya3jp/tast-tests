Bitstream file name: DPB_B_VIDYO_2.hevc

Bitstream feature name: Sublayer DPB info present

Bitstream feature description:
The VPS has sublayer DPB size structure: dpb_size(). For each sublayer (up to the max value sublayers for any layer) within each OLS, sublayer DPB info maybe present.
If present, it signals the max number of decoded pictures and output pictures reordering information.
max_vps_dec_pic_buffering_minus1[ i ][ k ][ j ] plus 1: when NecessaryLayerFlag[ i ][ k ]=1, it specifies the maximum number of decoded pictures, of the k-th layer for the CVS in the i-th OLS, that need to be stored in the DPB when HighestTid=j.
max_vps_num_reorder_pics[ i ][ j ]: when HighestTid=j, it specifies the maximum allowed number of access units containing a picture with PicOutputFlag=1 that can precede any access unit auA that contains a picture with PicOutputFlag=1 in the i-th OLS, in the CVS in decoding order and follow the access unit auA that contains a picture with PicOutputFlag=1 in output order.
max_vps_latency_increase_plus1[ i ][ j ]: used to compute the value of VpsMaxLatencyPictures[ i ][ j ].
VpsMaxLatencyPictures[ i ][ j ]: when HighestTid=j, it specifies the maximum number of access units containing a picture with PicOutputFlag=1 in the i-th OLS that can precede any access unit auA that contains a picture with PicOutputFlag=1 in the CVS in output order and follow the access unit auA that contains a picture with PicOutputFlag=1 in decoding order.

This bitstream has 3 temporal layers and Max sublayer DPB size varies for all sublayers.
Various parameters in the dpb_size() structure are as follows
- OLS 1 (not specified for default OLS 0) [max OLS = 2] (i = 1)
    sub_layer_flag_info_present_flag[ i ]: 1, i = 1
    Temporal sublayer 0 (j = 0)
	* for sublayer 0, sub_layer_dpb_info_present_flag[ i ][ j ] is not signalled, always 1
      max_vps_dec_pic_buffering_minus1[ i ][ k ][ j ]: 2, k = 0 to 1 [num layers in OLS_1 = 2, k = 0, 1]
      max_vps_num_reorder_pics[ i ][ j ]: 0
      max_vps_latency_increase_plus1[ i ][ j ]: 0
    Temporal sublayer 1 (j = 1)
      sub_layer_dpb_info_present_flag[ i ][ j ]: 1
      max_vps_dec_pic_buffering_minus1[ i ][ k ][ j ]: 3, k = 0 to 1 [num layers in OLS_1 = 2, k = 0, 1]
      max_vps_num_reorder_pics[ i ][ j ]: 0
      max_vps_latency_increase_plus1[ i ][ j ]: 0
    Temporal sublayer 2 (j = 2)
      sub_layer_dpb_info_present_flag[ i ][ j ]: 1
      max_vps_dec_pic_buffering_minus1[ i ][ k ][ j ]: 4, k = 0 to 1 [num layers in OLS_1 = 2, k = 0, 1]
      max_vps_num_reorder_pics[ i ][ j ]: 0
      max_vps_latency_increase_plus1[ i ][ j ]: 0

- Coding structure: Low Delay B
- Number of Frames - 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)

Number of layers: 2

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3.1

Each layer resolution:
- Layer 0 resolution - Coded:  960x544
                       Output: 960x540
- Layer 1 resolution - Coded:  1280x720

Frame rate: 30 fps for both layers

SHM Version: SHM Dev branch, rev 1527

Contact: Won Kap Jang, Vidyo Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

