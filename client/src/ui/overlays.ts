// Overlays that, while open, own the input (controls off, pointer free) besides
// the menu and the play modal (e.g. "Iniciá sesión"). Registered once at boot.
const extra: Array<() => boolean> = [];

export function registerOverlay(isOpen: () => boolean): void { extra.push(isOpen); }

export const extraOverlayOpen = (): boolean => extra.some((f) => f());
