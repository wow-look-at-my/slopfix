void FroxelPath::finishPrepack(Context& ctx) {
  // Frame 0 goes live through the ordinary adopt, which installs frame 0's
  // triple from the map.
  if (prepack_.empty()) {
    livePrepackFrame_ = kNoPrepackFrame;
    prepackPending_ = 0;
    adoptPrepacked(ctx);
  }
}
