/** Detect flexbox gap support (WebKitGTK on Astra reports @supports gap but flex gap fails). */
export function detectFlexGap(): boolean {
  if (typeof document === "undefined") return true;
  const flex = document.createElement("div");
  flex.style.display = "flex";
  flex.style.flexDirection = "column";
  flex.style.rowGap = "1px";
  flex.style.position = "absolute";
  flex.style.visibility = "hidden";
  flex.style.pointerEvents = "none";
  flex.appendChild(document.createElement("div"));
  flex.appendChild(document.createElement("div"));
  document.body.appendChild(flex);
  const supported = flex.scrollHeight === 1;
  document.body.removeChild(flex);
  return supported;
}

export function applyFlexGapClass(): void {
  if (!detectFlexGap()) {
    document.documentElement.classList.add("no-flex-gap");
  }
}
