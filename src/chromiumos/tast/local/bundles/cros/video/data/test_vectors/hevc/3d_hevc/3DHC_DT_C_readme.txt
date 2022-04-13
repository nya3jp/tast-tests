Name: 3DHC_DT_C
Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to ARP, sub-PU inter-view motion prediction and illumination compensation, DBBP is enabled for non-base texture views.

Functional stage: Decoding of three views with DBBP enabled for texture views.
Purpose: To conform the inter DBBP for texture.
frame number:64
Software: HTM15.1
Configure(base): baseCfg_3view+depth.cfg + qpCfg_Nview+depth_QP40.cfg + seqCfg_Kendo.cfg
  Configure: --IvMvPredFlag 1 1 --IvResPredFlag 1 --IlluCompEnable 1 --IlluCompLowLatencyEnc 0 --ViewSynthesisPredFlag 0 --DepthRefinementFlag 0 --IvMvScalingFlag 1 --Log2SubPbSizeMinus3 0 --Log2MpiSubPbSizeMinus3 0 --DepthBasedBlkPartFlag 1 --VSO 1 --IntraWedgeFlag 0 --IntraContourFlag 0 --IntraSdcFlag 0 --DLT 0 --QTL 0 --QtPredFlag 0 --InterSdcFlag 0 --MpiFlag 0 --DepthIntraSkip 0