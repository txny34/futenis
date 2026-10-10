import { bestMedals, CHALLENGES, MEDALS } from "../profile/challenges";
import { el } from "./dom";
import { t } from "../i18n/t";

// Entrenamiento setup: the training challenges and the best medal of each.
const NONE = "—";

export function challengesCard(): HTMLElement {
  const c = el("section", "fx-tcard fx-chal");
  c.append(el("h3", "fx-tcard__title", t("Desafíos", "Challenges")));
  const list = el("ul", "fx-chal__list");
  const saved = bestMedals();
  for (const def of CHALLENGES) {
    const best = saved[def.id] ?? -1;
    const li = el("li", "fx-chal__item");
    const goals = def.tiers.join(" / ");
    li.append(
      el("span", "fx-chal__title", def.title),
      el("span", "fx-chal__goals", goals, { "aria-label": t(`Bronce ${def.tiers[0]}, Plata ${def.tiers[1]}, Oro ${def.tiers[2]}`, `Bronze ${def.tiers[0]}, Silver ${def.tiers[1]}, Gold ${def.tiers[2]}`) }),
      el("span", `fx-chal__medal fx-chal__medal--${best >= 0 ? best : "none"}`, best >= 0 ? MEDALS[best] : NONE),
    );
    list.append(li);
  }
  c.append(list, el("p", "fx-note", t("Bronce / Plata / Oro en una sola sesión. Se guarda la mejor medalla.", "Bronze / Silver / Gold in a single session. The best medal is saved.")));
  return c;
}
