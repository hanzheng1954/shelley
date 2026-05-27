import type React from "react";

/**
 * Returns true if a mouse event has a modifier indicating the user wants to
 * open the action's target in a new tab/window (cmd/ctrl/shift/meta).
 */
export function isOpenInNewTabClick(e: React.MouseEvent): boolean {
  return e.metaKey || e.ctrlKey || e.shiftKey;
}

/**
 * If the click has a "new tab" modifier, open `url` in a new tab and return
 * true (so callers can early-return). Otherwise returns false and the caller
 * should perform its normal SPA action.
 */
export function handleModifiedNavClick(e: React.MouseEvent, url: string): boolean {
  if (!isOpenInNewTabClick(e)) return false;
  e.preventDefault();
  e.stopPropagation();
  window.open(url, "_blank", "noopener");
  return true;
}
