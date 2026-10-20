void FroxelPath::finishPrepack(Context& ctx) {
  ensureSplatCapacity(ctx, peakC);
  // The first frame goes live through the ordinary adopt.
  livePrepackFrame_ = kNoPrepackFrame;
  prepackPending_ = 0;
  cubeCount_ = 0;
  adoptPrepacked(ctx);
}
