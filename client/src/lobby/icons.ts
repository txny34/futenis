// Íconos de la barra del lobby: SVG en línea dibujados a mano (24x24, trazo
// redondeado, color = currentColor). Sin assets externos.
const NS = "http://www.w3.org/2000/svg";

export type IconName = "team" | "styles" | "missions" | "gift" | "shop" | "merch";

const PATHS: Readonly<Record<IconName, readonly string[]>> = {
  // Dos jugadores.
  team: ["M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6z", "M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6", "M16 11a2.5 2.5 0 1 0 0-5", "M17.5 14.2c2 .7 3.5 2.9 3.5 5.8"],
  // Paleta de pintor.
  styles: ["M12 3a9 9 0 1 0 0 18c1.2 0 1.8-.9 1.4-1.9-.5-1.1.2-2.1 1.4-2.1H17a4 4 0 0 0 4-4c0-5.5-4-10-9-10z", "M7.5 11.5h.01", "M10 7.5h.01", "M14.5 7.5h.01"],
  // Pergamino con tilde.
  missions: ["M6 4h11a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5", "M5 5a1 1 0 0 1 2 0v13", "M9.5 12l2 2 3.5-4"],
  // Caja de regalo.
  gift: ["M4 10h16v10H4z", "M3 7h18v3H3z", "M12 7v13", "M12 7c-1-2.5-4.5-3.5-4.5-1.2C7.5 7 10 7 12 7z", "M12 7c1-2.5 4.5-3.5 4.5-1.2C16.5 7 14 7 12 7z"],
  // Carrito.
  shop: ["M3 4h2l2.2 10.5a1 1 0 0 0 1 .8h9.3a1 1 0 0 0 1-.8L20 8H6.2", "M9 20h.01", "M17 20h.01"],
  // Remera.
  merch: ["M8 4l-5 3 2 4 3-1.5V20h8V9.5l3 1.5 2-4-5-3c-.5 1.5-2 2.5-4 2.5S8.5 5.5 8 4z"],
};

export function icon(name: IconName): SVGSVGElement {
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("class", "fx-icon");
  for (const d of PATHS[name]) {
    const p = document.createElementNS(NS, "path");
    p.setAttribute("d", d);
    svg.append(p);
  }
  return svg;
}
