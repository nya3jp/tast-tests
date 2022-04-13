Bitstream file name: REFLAYER_A_VIDYO_2.hevc

Bitstream feature name: Multiple active ref layers with various inter layer predictors

Bitstream feature description:
VPS Extension signals direct_dependency_flag[ i ][ j ] which specifies if the layer with index j is a direct reference layer for the layer with index i.
NumDirectRefLayers calculated as below can be greater than 1 for multi-layer bitstreams.
direct_dependency_all_layers_type indicates the type of dependency between each layer and it's reference layer.
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

Some important params in the VPS Extension are as below:
- Layer 1 (i = 1):
    direct_dependency_flag[ i ][ j ]: 1, i = 1, j = 0
	NumDirectRefLayers = 1
- Layer 2 (i = 2):
    direct_dependency_flag[ i ][ j ]: 1, i = 2, j = 0
    direct_dependency_flag[ i ][ j ]: 1, i = 2, j = 1
	NumDirectRefLayers = 2
- max_one_active_ref_layer_flag: 0
- direct_dependency_all_layers_flag: 1
- direct_dependency_all_layers_type: 2 (inter-layer sample prediction & inter-layer motion prediction)
Some important params in the Slice segment header are as below:
Frame 2 (POC id = 2), Frame 4 (POC id = 4), Frame 6 (POC id = 6), Frame 8 (POC id = 8)
- Layer 1: NumDirectRefLayers = 1
- Layer 2: NumDirectRefLayers = 2
    num_inter_layer_ref_pics_minus1: 0 (+1 = 1)
    inter_layer_pred_layer_idc[ i ]: 0, i = 0 (Layer 1 is used for prediction of Layer 2)
Frame 0 (POC id = 0), Frame 1 (POC id = 1), Frame 3 (POC id = 3), Frame 5 (POC id = 5), Frame 7 (POC id = 7)
- Layer 1: NumDirectRefLayers = 1
- Layer 2: NumDirectRefLayers = 2
    num_inter_layer_ref_pics_minus1: 0 (+1 = 1)
    inter_layer_pred_layer_idc[ i ]: 1, i = 0 (Layer 0 is used for prediction of Layer 2)
- Coding structure: Low Delay B
- Number of Frames - 9

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Main 3)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3)
- OLS_2 -   Layer: 0, PTL Idx: 1 (Main 3)
            Layer: 1, PTL Idx: 2 (Scalable Main 3)
            Layer: 2, PTL Idx: 3 (Scalable Main 4)

Number of layers: 3

Profile, Tier, Level information: Num PTL = 4
- PTL_Idx 0 - Profile: MAIN [V1 Whole Bitstream PTL]
              Level: 3.1
- PTL_Idx 1 - Profile: MAIN [Base layer PTL]
              Level: 3
- PTL_Idx 2 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 3
- PTL_Idx 3 - Profile: SCALABLE MAIN [Enhancement layer PTL]
              Level: 4

Each layer resolution:
- Layer 0 resolution - Output: 960x540
                       Coded:  960x544
- Layer 1 resolution - Output: 960X540
                       Coded:  960x544
- Layer 2 resolution - Output: 1920x1080
                       Coded:  1920x1080
					
Frame rate: 24 fps for both layers

This bitstream is generated using SHM rev 1527

Contact: Won Kap Jang, Vidyo, Inc. (wonkap@vidyo.com)
         Jay Padia, Vidyo Inc. (jpadia@vidyo.com)

