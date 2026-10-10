import { displayName } from "../../profile/display-name";
import { rankOf } from "../../profile/rank";
import { profileStore } from "../../profile/store";
import { el } from "../dom";
import { t } from "../../i18n/t";

// Left "Tu perfil" card of "Elegí un modo". Re-rendered on every open (no per-frame work).
export class ProfileSummary {
  readonly root = el("section", "fx-play__profile", "", { "aria-label": t("Tu perfil", "Your profile") });
  private readonly initial = el("span", "fx-profile__initial");
  private readonly level = el("span", "fx-profile__level");
  private readonly name = el("b", "fx-play__name");
  private readonly record = el("span", "fx-play__record");
  private readonly elo1 = el("b");
  private readonly elo2 = el("b");

  constructor() {
    const avatar = el("div", "fx-profile__avatar", "", { "aria-hidden": "true" });
    avatar.append(this.initial, this.level);
    const who = el("div", "fx-play__who");
    who.append(this.name, this.record);
    const head = el("div", "fx-play__head");
    head.append(avatar, who);
    const elos = el("dl", "fx-play__elos");
    const d1 = el("dd");
    const d2 = el("dd");
    d1.append(this.elo1);
    d2.append(this.elo2);
    elos.append(el("dt", "", "ELO 1v1"), d1, el("dt", "", "ELO 2v2"), d2);
    this.root.append(el("h3", "fx-play__heading", t("Tu perfil", "Your profile")), head, elos);
  }

  refresh(): void {
    const s = profileStore().get();
    this.initial.textContent = displayName(s.name).charAt(0).toUpperCase();
    this.level.textContent = String(s.level);
    this.name.textContent = displayName(s.name);
    this.record.textContent = t(`Nivel ${s.level} · ${s.wins}G ${s.losses}P`, `Level ${s.level} · ${s.wins}W ${s.losses}L`);
    this.elo1.textContent = `${rankOf(s.elo["1v1"]).label} · ${s.elo["1v1"]}`;
    this.elo2.textContent = `${rankOf(s.elo["2v2"]).label} · ${s.elo["2v2"]}`;
  }
}
