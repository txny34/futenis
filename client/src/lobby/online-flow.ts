import { t } from "../i18n/t";
import { NetStatus, RemoteEngine } from "../net/remote-engine";
import { serverBase } from "../net/server-url";
import { recorder } from "../profile/recorder";
import type { QueueMode } from "../ui/play-modal/play-modal";
import { arm, landingAllowed, type App } from "./flow";
import { cancelIntro } from "./intro";
import { endReplay } from "./replay-state";
import { leaveOnline, setRemote } from "./online-state";

// Online casual match from "Elegí un modo": the server pairs whoever is queued
// (cmd/server/queue.go) and the loop renders its frames instead of the local sim.
// Casual: no ELO, no profile stats, no replay (the tape needs the local sim).

function statusText(s: NetStatus): string {
  switch (s) {
    case "conectando": return t("Conectando…", "Connecting…");
    case "esperando rival": return t("Buscando rival…", "Finding an opponent…");
    case "jugando": return "";
    case "el rival se fue": return t("El rival se fue", "Your opponent left");
    case "nadie se unió a la sala": return t("No apareció ningún rival. Probá de nuevo.", "No opponent showed up. Try again.");
    case "versión distinta, recargá": return t("Hay una versión nueva: recargá la página", "New version available: reload the page");
    case "desconectado por inactividad": return t("Desconectado por inactividad", "Disconnected due to inactivity");
    case "demasiados mensajes": return t("Desconectado: demasiados mensajes", "Disconnected: too many messages");
    default: return t("Desconectado del servidor", "Disconnected from the server");
  }
}

export function startOnline(app: App, mode: QueueMode): void {
  if (!app.loop) return;
  leaveOnline(app);
  endReplay(app);
  cancelIntro();
  recorder().end();
  const remote = new RemoteEngine(`${serverBase()}/ws?queue=${mode}`, 0);
  remote.onStatus = (s, ping) => app.hud.error(s === "jugando" ? `${ping} ms` : statusText(s));
  setRemote(remote);
  app.mode = mode;
  app.last = null; // Revancha requeues (see showResult); never a bot rematch
  app.state.holdUntil = 0;
  app.state.showLanding = landingAllowed(app.menu.settings, null);
  arm(app, false);
  app.loop.setEngine(remote);
  app.controls.lock();
  remote.connect();
}
