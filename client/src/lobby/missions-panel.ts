import { MissionView, missions, Period } from "../profile/missions";
import { el } from "../ui/dom";

// Panel "Misiones diarias" (izquierda, plegable) con pestañas Día/Sem. Se
// reconstruye solo cuando cambian las misiones o la pestaña, nunca por frame.
const TABS: ReadonlyArray<readonly [Period, string]> = [["day", "Día"], ["week", "Sem."]];
const TITLE: Readonly<Record<Period, string>> = { day: "Misiones diarias", week: "Misiones semanales" };
const FOOT: Readonly<Record<Period, string>> = { day: "Nuevas misiones a medianoche", week: "Nuevas misiones el lunes" };

export class MissionsPanel {
  readonly root = el("aside", "fx-missions", "", { "aria-label": "Misiones" });
  private readonly title = el("b", "fx-missions__title");
  private readonly list = el("ul", "fx-missions__list");
  private readonly foot = el("p", "fx-missions__foot");
  private readonly toggle = el("button", "fx-missions__toggle", "«", { type: "button", "aria-expanded": "true" });
  private readonly tabEls: HTMLButtonElement[] = [];
  private tab: Period = "day";

  constructor(parent: HTMLElement) {
    const head = el("header", "fx-missions__head");
    const tabs = el("div", "fx-chips fx-missions__tabs", "", { role: "tablist" });
    for (const [p, label] of TABS) {
      const b = el("button", "fx-chip", label, { type: "button", role: "tab", "aria-selected": String(p === this.tab) });
      b.addEventListener("click", () => { this.select(p); b.blur(); });
      this.tabEls.push(b);
      tabs.append(b);
    }
    this.toggle.addEventListener("click", () => { this.collapsed = !this.collapsed; this.toggle.blur(); });
    head.append(this.toggle, this.title, tabs);
    this.root.append(head, this.list, this.foot);
    this.root.hidden = true;
    parent.append(this.root);
    missions().onChange(() => this.render());
    this.render();
  }

  set visible(v: boolean) { this.root.hidden = !v; }
  get visible(): boolean { return !this.root.hidden; }

  get collapsed(): boolean { return this.root.classList.contains("is-collapsed"); }
  set collapsed(v: boolean) {
    this.root.classList.toggle("is-collapsed", v);
    this.toggle.textContent = v ? "»" : "«";
    this.toggle.setAttribute("aria-expanded", String(!v));
  }

  private select(p: Period): void {
    this.tab = p;
    this.tabEls.forEach((b, i) => b.setAttribute("aria-selected", String(TABS[i][0] === p)));
    this.render();
  }

  private render(): void {
    this.title.textContent = TITLE[this.tab];
    this.foot.textContent = FOOT[this.tab];
    this.list.replaceChildren(...missions().list(this.tab).map((m) => this.row(m)));
  }

  private row(m: MissionView): HTMLLIElement {
    const li = el("li", m.claimed ? "fx-mission is-claimed" : "fx-mission");
    const bar = el("span", "fx-mission__bar");
    const fill = el("span", "fx-mission__fill");
    fill.style.width = `${Math.round((100 * m.progress) / m.def.goal)}%`;
    bar.append(fill);
    const end = m.claimable
      ? el("button", "fx-btn fx-mission__claim", `+${m.def.xp} XP`, { type: "button" })
      : el("span", "fx-mission__reward", m.claimed ? "✓" : `+${m.def.xp} XP`);
    if (end instanceof HTMLButtonElement) end.addEventListener("click", () => missions().claim(this.tab, m.def.id));
    li.append(el("span", "fx-mission__name", m.def.title), el("span", "fx-mission__count", `${m.progress}/${m.def.goal}`), end, bar);
    return li;
  }
}
