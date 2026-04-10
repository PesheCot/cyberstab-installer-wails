import React, { useEffect, useState } from "react";

function cls(...parts: Array<string | false | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export const UninstallApp: React.FC = () => {
  const [installDir, setInstallDir] = useState("C:\\Program Files\\Cyberstab");
  const [pgPass, setPgPass] = useState("");
  const [skipDB, setSkipDB] = useState(false);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [report, setReport] = useState("");
  const [errorText, setErrorText] = useState("");

  useEffect(() => {
    const h = (payload: any) => {
      setReport(String(payload?.report ?? ""));
      setDone(true);
      setBusy(false);
    };
    window.runtime?.EventsOn?.("uninstall:done", h);
    return () => {
      try {
        window.runtime?.EventsOff?.("uninstall:done");
      } catch {
        // ignore
      }
    };
  }, []);

  const onUninstall = async () => {
    setErrorText("");
    setDone(false);
    setReport("");
    setBusy(true);
    try {
      await window.go.main.App.Uninstall({
        installDir: installDir.trim(),
        postgresPassword: skipDB ? "" : pgPass,
        skipDB,
      });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
      setBusy(false);
    }
  };

  return (
    <div className="app">
      <div className="topBar">
        <div className="topBarInner" style={{ ["--wails-draggable" as any]: "drag" }}>
          <div className="topLeft">
            <div className="topLogo">C</div>
            <div className="topTitle">Киберстаб — удаление</div>
          </div>
          <div className="topWinButtons" style={{ ["--wails-draggable" as any]: "no-drag" }}>
            <button className="winBtn" onClick={() => window.runtime.WindowMinimise()} aria-label="Minimise">
              —
            </button>
            <button className="winBtn" onClick={() => window.runtime.WindowToggleMaximise()} aria-label="Maximise">
              □
            </button>
            <button className="winBtn winClose" onClick={() => window.runtime.Quit()} aria-label="Close">
              ×
            </button>
          </div>
        </div>
      </div>

      <div className="content">
        <div className="card">
          <div className="cardTitle">Удаление Cyberstab</div>
          <div className="hint">
            Будут удалены: папка установки, база <b>okidoci_db</b> и роли <b>okidoci_*</b>, <b>oki_*</b>, задачи планировщика и ярлыки.
            Сам PostgreSQL не удаляется.
          </div>

          <div className="cardTitle" style={{ marginTop: 14 }}>
            Папка установки
          </div>
          <div className="row">
            <input className="input" value={installDir} onChange={(e) => setInstallDir(e.target.value)} disabled={busy} />
          </div>

          <div className="cardTitle" style={{ marginTop: 14 }}>
            PostgreSQL
          </div>
          <label className="check" style={{ marginBottom: 10 }}>
            <input type="checkbox" checked={skipDB} onChange={(e) => setSkipDB(e.target.checked)} disabled={busy} />
            <span>Не трогать базу данных (только файлы и Windows)</span>
          </label>
          {!skipDB && (
            <div className="row">
              <input
                className="input"
                type="password"
                value={pgPass}
                onChange={(e) => setPgPass(e.target.value)}
                placeholder="Пароль пользователя postgres"
                disabled={busy}
              />
            </div>
          )}

          {errorText && (
            <div className="hint" style={{ marginTop: 12, color: "#b42318" }}>
              {errorText}
            </div>
          )}

          {done && report && (
            <div className="hint" style={{ marginTop: 12 }}>
              {report}
            </div>
          )}

          <div className="wizardFooter" style={{ marginTop: 18 }}>
            <button className={cls("btnPrimary", busy && "disabled")} onClick={onUninstall} disabled={busy || (!skipDB && pgPass.trim().length === 0)}>
              {busy ? "Удаление…" : "Удалить"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
