Bitstream file name: REFLAYER_D_VIDYO_2.hevc

Bitstream feature name: Multiple active ref layers with various inter layer predictors

Bitstream feature description:
VPS Extension signals direct_dependency_flag[ i ][ j ] which specifies if the layer with index j is a direct reference layer for the layer with index i. NumDirectRefLayers calculated as below can be greater than 1 for multi-layer bitstreams.
direct_dependency_type[ i ][ j ] indicates the type of dependency between the layer with nuh_layer_id equal layer_id_in_nuh[ i ] and the layer with nuh_layer_id equal to layer_id_in_nuh[ j ].
num_inter_layer_ref_pics_minus1+1 (slice segment header, annex F) specifies the number of pictures that may be used in decoding of the current picture for inter-layer prediction.
According to annex H, for Scalable Main & Scalable Main 10 profiles, the variables NumRefLayerPicsProcessing, NumRefLayerPicsSampleProcessing, and NumRefLayerPicsMotionProcessing shall be less than or equal to 1 for each decoded picture (effectively, max 1 reference layer for Scalable profiles).
NumDirectRefLayers (F-4)
  for( j = 0, d = 0; j <= MaxLayersMinus1; j++ )
  {
    jNuhLid = layer_id_in_nuh[ j ];
    if( direct_dependency_flag[ i ][ j ] )
  	  IdDirectRefLayer[ iNuhLId ][ d++ ] = jNuhLid;
  }
  NumDirectRefLayers = d;
In this bitstream, NumDirectRefLayers=2 for Layer 2. Various frames may use either Layer 0 or Layer 1 for sample prediction or motion prediction.
This bitstream uses direct_dependency_all_layers_type in the VPS instead of signalling direct_dependency_type for each reference layer.
Each spatial layer in the bitstream has 4 temporal layers. The max temporal layer () available for inter-layer prediction is temporal layer 2.
The GOP size is 8. The 8th picture which is the first frame in encoding order in the GOP, only uses inter-layer prediction for ELs.

Temporal Layer structure for the bitstream
Frame 0 (POC id = 0), Frame 1 (POC id = 8): Temporal sub-layer id = 0
Frame 2 (POC id = 4) : Temporal sub-layer id = 1
Frame 3 (POC id = 2), Frame 6 (POC id = 6): Temporal sub-layer id = 2
Frame 4 (POC id = 1), Frame 5 (POC id = 3), Frame 7 (POC id = 5), Frame 8 (POC id = 7): Temporal sub-layer id = 3
Some important params in the VPS Extension are as below:
- Layer 1 (i = 1):
    direct_dependency_flag[ i ][ j ]: 1, i = 1, j = 0
	NumDirectRefLayers = 1
- Layer 2 (i = 2):
    direct_dependency_flag[ i ][ j ]: 1, i = 2, j = 0
    direct_dependency_flag[ i ][ j ]: 1, i = 2, j = 1
	NumDirectRefLayers = 2
- max_tid_ref_present_flag: 1
- Layer 0 (i = 0)
    max_tid_il_ref_pics_plus1[ i ][ j ]: 3, i = 0, j = 1
    max_tid_il_ref_pics_plus1[ i ][ j ]: 3, i = 0, j = 2
- Layer 1 (i = 1)
    max_tid_il_ref_pics_plus1[ i ][ j ]: 3, i = 1, j = 2
- max_one_active_ref_layer_flag: 0
- direct_dependency_all_layers_flag: 1
- direct_dependency_all_layers_type: 2 (inter-layer sample prediction & inter-layer motion prediction allowed for all reference layers)
Some important params in the Slice segment header are as below:
Frame 0 (POC id = 0), Frame 1 (POC id = 8), Frame 3 (POC id = 2)
- Layer 1: NumDirectRefLayers = 1
- Layer 2: NumDirectRefLayers = 2
    num_inter_layer_ref_pics_minus1: 0 (+1 = 1)
    inter_layer_pred_layer_idc[ i ]: 0, i = 0 (Layer 0 is used for prediction of Layer 2)
Frame 2 (POC id = 4), Frame 6 (POC id = 6)
- Layer 1: NumDirectRefLayers = 1
- Layer 2: NumDirectRefLayers = 2
    num_inter_layer_ref_pics_minus1: 0 (+1 = 1)
    inter_layer_pred_layer_idc[ i ]: 1, i = 0 (Layer 1 is used for prediction of Layer 2)
Frame 4 (POC id = 1), Frame 5 (POC id = 3), Frame 7 (POC id = 5), Frame 8 (POC id = 7)
- Layer 1 & Layer 2: inter_layer_pred_enabled_flag: false (Temporal Sub-layer id = 3, which is greater than max_tid_il_ref_pics_plus1)
- Coding structure: Random Access B
- Number of Frames - 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3.1)
            Layer: 2, PTL Idx: 3 (Scalable Main 4)

Number of layers: 3

Profile, Tier, Level information: Num PTL = 4
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3.1
- PTL_Idx 3 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 960x540
                       Coded:  960x544
- Layer 1 resolution - Output: 1280X720
                       Coded:  1280x720
- Layer 2 resolution - Output: 1920x1080
                       Coded:  1920x1080
					
Frame rate: 24 fps for all 3 layers

This bitstream is generated using SHM rev 1087

Contact: Won Kap Jang, Vidyo Inc. (wonkap@vidyo.com)
         Jill Boyce, Vidyo Inc. (jill@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

