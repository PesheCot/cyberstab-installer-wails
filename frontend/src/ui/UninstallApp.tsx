import React, { useEffect, useState } from "react";
import cyberstabLogo from "../assets/cyberstab-logo.svg";

function cls(...parts: Array<string | false | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export const UninstallApp: React.FC = () => {
  const [dbEngines, setDbEngines] = useState<Array<{ kind: "postgresql" | "jatoba"; label: string; binDir: string }>>([]);
  const [dbEngineKind, setDbEngineKind] = useState<"postgresql" | "jatoba" | "">("");
  const [installDir, setInstallDir] = useState("C:\\Program Files\\Cyberstab");
  const [pgUser, setPgUser] = useState("postgres");
  const [pgPass, setPgPass] = useState("");
  const [showPgPass, setShowPgPass] = useState(false);
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

  useEffect(() => {
    (async () => {
      try {
        const result = await window.go.main.App.CheckDbInstalled();
        const engines = (result?.engines || []) as Array<{ kind: "postgresql" | "jatoba"; label: string; binDir: string }>;
        setDbEngines(engines);
        if (engines.length === 1) {
          setDbEngineKind(engines[0].kind);
          await window.go.main.App.SelectDbEngine(engines[0].kind);
        } else if (engines.length > 1) {
          const preferred = engines.find((e) => e.kind === "postgresql")?.kind || engines[0].kind;
          setDbEngineKind(preferred);
          await window.go.main.App.SelectDbEngine(preferred);
        }
      } catch {
        // ignore
      }
    })();
  }, []);

  const onUninstall = async () => {
    setErrorText("");
    setDone(false);
    setReport("");
    setBusy(true);
    try {
      if (!skipDB && dbEngineKind) {
        await window.go.main.App.SelectDbEngine(dbEngineKind);
      }
      await window.go.main.App.Uninstall({
        installDir: installDir.trim(),
        dbEngine: skipDB ? "" : dbEngineKind,
        postgresUser: skipDB ? "" : pgUser,
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
            <img className="topLogo" src={cyberstabLogo} alt="" aria-hidden />
            <div className="topTitle">Киберстаб — удаление</div>
          </div>
          <div className="topWinButtons" style={{ ["--wails-draggable" as any]: "no-drag" }}>
            <button className="winBtn" onClick={() => window.runtime.WindowMinimise()} aria-label="Minimise">
              —
            </button>
            <button className="winBtn winClose" onClick={() => window.runtime.Quit()} aria-label="Close">
              ×
            </button>
          </div>
        </div>
      </div>

      <div className="content">
        <div className="wizardPanel uninstallPanel">
          <div className="wizardHead">
            <div>
              <h1 className="wizardTitle">Мастер удаления</h1>
              <p className="wizardStep">Удаление Киберстаб</p>
            </div>
            <p className="wizardOs">ОС: Windows</p>
          </div>

          <h2 className="wizardSectionTitle">Что удалить</h2>
          <div className="wizardBody">
            <p className="wizardHint uninstallLead">
              Будут удалены папка установки, база <b>okidoci_db</b>, роли <b>okidoci_*</b>, <b>oki_*</b>, задачи планировщика и ярлыки.
              Сам PostgreSQL не удаляется.
            </p>

            <h3 className="uninstallSectionTitle">Папка установки</h3>
            <div className="pathRow uninstallPathRow">
              <input className="input uninstallInput" value={installDir} onChange={(e) => setInstallDir(e.target.value)} disabled={busy} />
            </div>

            <h3 className="uninstallSectionTitle">PostgreSQL</h3>
            {dbEngines.length > 1 && (
              <div className="radioList" style={{ marginTop: 0, marginBottom: 10 }}>
                <label className={cls("radioRow", dbEngineKind === "postgresql" && "radioRowOn", busy && "disabled")}>
                  <input
                    type="radio"
                    name="uninstallDbEngine"
                    checked={dbEngineKind === "postgresql"}
                    onChange={async () => {
                      setDbEngineKind("postgresql");
                      await window.go.main.App.SelectDbEngine("postgresql");
                    }}
                    disabled={busy}
                  />
                  <span className="radioRowDot" />
                  <span>PostgreSQL</span>
                </label>
                <label className={cls("radioRow", dbEngineKind === "jatoba" && "radioRowOn", busy && "disabled")}>
                  <input
                    type="radio"
                    name="uninstallDbEngine"
                    checked={dbEngineKind === "jatoba"}
                    onChange={async () => {
                      setDbEngineKind("jatoba");
                      await window.go.main.App.SelectDbEngine("jatoba");
                    }}
                    disabled={busy}
                  />
                  <span className="radioRowDot" />
                  <span>Jatoba</span>
                </label>
              </div>
            )}
            <label className={cls("radioRow uninstallToggle", skipDB && "radioRowOn", busy && "disabled")}>
              <input type="checkbox" checked={skipDB} onChange={(e) => setSkipDB(e.target.checked)} disabled={busy} />
              <span className="radioRowDot" />
              <span>Не трогать базу данных (только файлы и Windows)</span>
            </label>

            {!skipDB && (
              <>
                {dbEngines.length === 1 && (
                  <p className="wizardHint" style={{ margin: "0 0 10px" }}>
                    Найдена СУБД: <b>{dbEngines[0]?.kind === "jatoba" ? "Jatoba" : "PostgreSQL"}</b>.
                  </p>
                )}
                <div className="pgCredentialGrid uninstallPgRow">
                  <div className="pgCredentialLine">
                    <div className={cls("passwordField", errorText && "inputError")}>
                      <input
                        className="input passwordInput"
                        type="text"
                        value={pgUser}
                        onChange={(e) => {
                          setPgUser(e.target.value);
                          setErrorText("");
                        }}
                        placeholder="Пользователь PostgreSQL"
                        disabled={busy}
                        autoComplete="username"
                      />
                    </div>
                  </div>
                  <div className="pgCredentialLine">
                    <div className={cls("passwordField", errorText && "inputError")}>
                      <input
                        className="input passwordInput"
                        type={showPgPass ? "text" : "password"}
                        value={pgPass}
                        onChange={(e) => {
                          setPgPass(e.target.value);
                          setErrorText("");
                        }}
                        placeholder="Пароль"
                        disabled={busy}
                      />
                      <button
                        type="button"
                        className="passwordEye"
                        onClick={() => setShowPgPass((v) => !v)}
                        disabled={busy}
                        aria-label={showPgPass ? "Скрыть пароль" : "Показать пароль"}
                      >
                        <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                          <path d="M2.8 12s3.4-6 9.2-6 9.2 6 9.2 6-3.4 6-9.2 6-9.2-6-9.2-6Z" />
                          <circle cx="12" cy="12" r="2.6" />
                        </svg>
                      </button>
                    </div>
                    {dbEngines.length === 1 && (
                      <button
                        type="button"
                        className={cls("btnLink", busy && "disabled")}
                        onClick={async () => {
                          try {
                            const dir = await window.go.main.App.PickDbDir();
                            if (!dir) return;
                            const result = await window.go.main.App.CheckDbInstalled();
                            const engines = (result?.engines || []) as Array<{ kind: "postgresql" | "jatoba"; label: string; binDir: string }>;
                            setDbEngines(engines);
                            if (engines.length > 1 && !dbEngineKind) {
                              const preferred = engines.find((e) => e.kind === "postgresql")?.kind || engines[0].kind;
                              setDbEngineKind(preferred);
                            }
                          } catch (e: any) {
                            setErrorText(String(e?.message || e));
                          }
                        }}
                        disabled={busy}
                      >
                        Добавить вторую СУБД по пути…
                      </button>
                    )}
                  </div>
                </div>
              </>
            )}

            {errorText && <p className="wizardHint wizardHintError uninstallMessage">{errorText}</p>}
            {done && report && <p className="wizardHint wizardHintSuccess uninstallMessage">{report}</p>}
          </div>
        </div>
      </div>

      <div className="footerBar">
        <div className="footerInner">
          <button type="button" className={cls("btnPrimary", busy && "disabled")} onClick={() => window.runtime.Quit()} disabled={busy}>
            Отмена
          </button>
          <div className="footerRight">
            <button className={cls("btnDanger", busy && "disabled")} onClick={onUninstall} disabled={busy || (!skipDB && (dbEngineKind.trim().length === 0 || pgUser.trim().length === 0 || pgPass.trim().length === 0))}>
              {busy ? "Удаление…" : "Удалить"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
