import { rankOf } from "../profile/rank";
import { profileStore, ProfileStats, type Queue } from "../profile/store";
import { el } from "../ui/dom";

// Tarjeta de perfil arriba a la derecha (solo en el lobby): avatar con nivel,
// nombre, barra de XP y ELO 1v1/2v2. Se toca el DOM solo en onChange del perfil.
export class ProfileCard {
  readonly root = el("section", "fx-profile", "", { "aria-label": "Perfil" });
  private readonly initial = el("span", "fx-profile__initial");
  private readonly level = el("span", "fx-profile__level");
  private readonly name = el("b", "fx-profile__name");
  private readonly fill = el("span", "fx-profile__fill");
  private readonly xp = el("span", "fx-profile__xp");
  private readonly elo1 = el("span", "fx-chip fx-profile__elo");
  private readonly elo2 = el("span", "fx-chip fx-profile__elo");

  constructor(parent: HTMLElement) {
    const avatar = el("div", "fx-profile__avatar", "", { "aria-hidden": "true" });
    avatar.append(this.initial, this.level);
    const bar = el("div", "fx-profile__bar", "", { role: "progressbar", "aria-label": "Experiencia" });
    bar.append(this.fill);
    const info = el("div", "fx-profile__info");
    const elos = el("div", "fx-profile__elos");
    elos.append(this.elo1, this.elo2);
    info.append(this.name, bar, this.xp, elos);
    this.root.append(avatar, info);
    this.root.hidden = true;
    parent.append(this.root);
    const store = profileStore();
    this.render(store.get());
    store.onChange((s) => this.render(s));
  }

  set visible(v: boolean) { this.root.hidden = !v; }

  private render(s: Readonly<ProfileStats>): void {
    this.initial.textContent = s.name.charAt(0).toUpperCase();
    this.level.textContent = String(s.level);
    this.name.textContent = s.name;
    this.fill.style.width = `${Math.round((100 * s.xp) / s.xpToNext)}%`;
    this.xp.textContent = `${s.xp} / ${s.xpToNext} XP`;
    showRank(this.elo1, "1v1", s.elo["1v1"]);
    showRank(this.elo2, "2v2", s.elo["2v2"]);
    this.root.title = `Nivel ${s.level} · ${s.wins}G ${s.losses}P · racha ${s.streak} · ${s.perfects} perfectos`;
  }
}

/** "1v1 · Oro II" chip tinted by tier, exact ELO in the tooltip. */
function showRank(chip: HTMLElement, queue: Queue, elo: number): void {
  const r = rankOf(elo);
  chip.textContent = `${queue} · ${r.label}`;
  chip.dataset.tier = r.tier;
  chip.title = `${queue}: ${elo} ELO`;
}
