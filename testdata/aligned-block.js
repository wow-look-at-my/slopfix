const reveal = (() => {
  /* Walks light DOM and open shadow roots alike, because the composer
     builds its field and button inside its own shadow root - a document-only
     query would light the standalone controls and miss those two. An element
     whose definition has not loaded yet has no shadow root to descend into, so
     it gets revisited once the definition lands. */
  function visit(el: Element): void {
    if (el.matches(SELECTOR)) track(el as HTMLElement);
    if (el.shadowRoot) descend(el.shadowRoot);
    else if (el.localName.includes('-') && !customElements.get(el.localName)) {
      customElements.whenDefined(el.localName)
        .then(() => { if (el.isConnected) visit(el); });
    }
  }
  return { visit };
})();
