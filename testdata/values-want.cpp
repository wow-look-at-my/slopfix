struct WarpPass {
  // Run answers one call multiple ways and ships hooks.
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
