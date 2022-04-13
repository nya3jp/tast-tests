Bitstream file name: SREXT_D_FUJITSU_1.hevc

Bitstream feature name: 2 Monochrome layers with a bit depth increase of 4 bits

Bitstream feature description:
2 Monochrome Layers coded with bitdepth scalability where layer 0 follows the Monochrome profile and layer 1 follows the Monochrome 12 profile.

Number of Layers: 2
- Layer 0	: 	QP0 = 22
- Layer 0	:	bit_depth_luma_minus8 = 0
- Layer 0	:	bit_depth_chroma_minus8 = 0
- Layer 0:	chroma_format_idc = 0
- Layer 1	: 	QP1 = 20
- Layer 1	:	bit_depth_luma_minus8 = 4
- Layer 1	:	bit_depth_chroma_minus8 = 4
- Layer 1:	chroma_format_idc = 0
- Layer 1	: 	NumSamplePredRefLayers1 = 1
- Layer 1	: 	SamplePredRefLayerIds1 = 0
- Layer 1	: 	NumMotionPredRefLayers1 = 1
- Layer 1	: 	MotionPredRefLayerIds1 = 0
- Layer 1	: 	NumActiveRefLayers1 = 1
- Layer 1	: 	PredLayerIds1 = 0

Coding structure: Low delay B
Number of Frames: 4

Output Layer Sets:
- OLS_0 -   Layer: 0, PTL Idx: 1 (Monochrome)
- OLS_1 -   Layer: 0, PTL Idx: 1 (Monochrome 12)
            Layer: 1, PTL Idx: 2 (Scalable Monochrome 12)
- DefaultTargetOutputLayerIdc   : 1

Profile, Tier, Level information: Num PTL = 3
- PTL_Idx 0 - Profile: MONOCHROME [Whole Bitstream PTL]
              Level: 4
- PTL_Idx 1 - Profile: MONOCHROME 12 [Base layer PTL]
              Level: 4
- PTL_Idx 2 - Profile: SCALABLE MONOCHROME 12 [Enhancement layer PTL]
              Level: 4
			
Each layer resolution:
- Layer 0 resolution - Output: 512x512, 8 bits, monochrome
                       Coded:  512x512, 8 bits
- Layer 1 resolution - Output: 512x512, 10 bits, monochrome
                       Coded:  512x512, 10 bits
					
Frame rate: 30 fps for both layers

SHM Version: SHM11 rev 1513

contact: Guillaume Barroux, Fujitsu Laboratories (guillaume.b@jp.fujitsu.com)