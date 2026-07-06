import React, { useEffect, useState } from "react";
import cyberstabLogo from "../assets/cyberstab-logo.svg";

type AppInfo = {
  os: string;
  needAdmin: boolean;
  postgresInstalled: boolean;
};

type DbEngineKind = "postgresql" | "postgrespro" | "jatoba";

type DbEngineInfo = {
  kind: DbEngineKind;
  label: string;
  binDir: string;
};

function cls(...parts: Array<string | false | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function osLabel(os?: string) {
  if (os === "windows") return "Windows";
  if (os === "linux") return "Linux";
  return os || "—";
}

function dbEngineLabel(kind?: string, engines?: DbEngineInfo[]) {
  const found = engines?.find((e) => e.kind === kind);
  if (found?.label) return found.label;
  if (kind === "jatoba") return "Jatoba";
  if (kind === "postgrespro") return "Postgres Pro";
  return "PostgreSQL";
}

function pickPreferredEngine(engines: DbEngineInfo[]): DbEngineInfo | undefined {
  return engines.find((e) => e.kind === "postgresql") ?? engines.find((e) => e.kind === "postgrespro") ?? engines[0];
}

const RadioRow: React.FC<{
  name: string;
  label: string;
  checked: boolean;
  onChange: () => void;
  disabled?: boolean;
}> = ({ name, label, checked, onChange, disabled }) => (
  <label className={cls("radioRow", checked && "radioRowOn", disabled && "disabled")}>
    <input type="radio" name={name} checked={checked} onChange={onChange} disabled={disabled} />
    <span className="radioRowDot" />
    <span>{label}</span>
  </label>
);

export const UninstallApp: React.FC = () => {
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [dbEngines, setDbEngines] = useState<DbEngineInfo[]>([]);
  const [dbEngineKind, setDbEngineKind] = useState<DbEngineKind | "">("");
  const [dbEngineBinDir, setDbEngineBinDir] = useState("");
  const [installDir, setInstallDir] = useState("");
  const [pgUser, setPgUser] = useState("postgres");
  const [pgPass, setPgPass] = useState("");
  const [showPgPass, setShowPgPass] = useState(false);
  const [skipDB, setSkipDB] = useState(false);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [report, setReport] = useState("");
  const [errorText, setErrorText] = useState("");
  const [showResetModal, setShowResetModal] = useState(false);
  const [resetError, setResetError] = useState("");
  const [newPgPass, setNewPgPass] = useState("");
  const [newPgPass2, setNewPgPass2] = useState("");
  const [resetBusy, setResetBusy] = useState(false);

  const isLinux = info?.os === "linux";

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
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
        if (i.os === "windows") setInstallDir("C:\\Program Files\\Cyberstab");
        if (i.os === "linux") setInstallDir("/opt/cyberstab");
      } catch {
        // ignore
      }
    })();
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const result = await window.go.main.App.CheckDbInstalled();
        const engines = (result?.engines || []) as DbEngineInfo[];
        setDbEngines(engines);
        if (engines.length === 1) {
          const e = engines[0];
          setDbEngineKind(e.kind);
          setDbEngineBinDir(e.binDir);
          await window.go.main.App.SelectDbEngineBin(e.binDir);
        } else if (engines.length > 1) {
          const preferred = pickPreferredEngine(engines);
          if (preferred) {
            setDbEngineKind(preferred.kind);
            setDbEngineBinDir(preferred.binDir);
            await window.go.main.App.SelectDbEngineBin(preferred.binDir);
          }
        } else {
          setDbEngineKind("");
          setDbEngineBinDir("");
        }
      } catch {
        // ignore
      }
    })();
  }, []);

  const onPickDbDir = async () => {
    setErrorText("");
    try {
      const dir = await window.go.main.App.PickDbDir();
      if (dir) {
        const result = await window.go.main.App.CheckDbInstalled();
        const engines = (result?.engines || []) as DbEngineInfo[];
        setDbEngines(engines);
        const selected = engines.find((e) => e.binDir.toLowerCase().includes(dir.toLowerCase())) ?? pickPreferredEngine(engines);
        if (selected) {
          setDbEngineKind(selected.kind);
          setDbEngineBinDir(selected.binDir);
          await window.go.main.App.SelectDbEngineBin(selected.binDir);
        }
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
      }
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    }
  };

  const onResetPassword = async () => {
    setResetError("");
    if (newPgPass.trim().length === 0) {
      setResetError("Новый пароль не должен быть пустым.");
      return;
    }
    if (newPgPass !== newPgPass2) {
      setResetError("Пароли не совпадают.");
      return;
    }
    const user = pgUser.trim();
    if (!user) {
      setResetError("Укажите пользователя PostgreSQL.");
      return;
    }
    setResetBusy(true);
    try {
      if (dbEngineBinDir) {
        await window.go.main.App.SelectDbEngineBin(dbEngineBinDir);
      } else if (dbEngineKind) {
        await window.go.main.App.SelectDbEngine(dbEngineKind);
      }
      await window.go.main.App.ResetPostgresPassword(user, newPgPass);
      setPgPass(newPgPass);
      try {
        await window.go.main.App.VerifyPostgresPassword(user, newPgPass);
        setErrorText("");
      } catch (e: any) {
        setErrorText(String(e?.message || e));
      }
      setShowResetModal(false);
      setNewPgPass("");
      setNewPgPass2("");
    } catch (e: any) {
      setResetError(String(e?.message || e));
    } finally {
      setResetBusy(false);
    }
  };

  const onUninstall = async () => {
    setErrorText("");
    setDone(false);
    setReport("");
    setBusy(true);
    try {
      if (!skipDB) {
        if (dbEngineBinDir) {
          await window.go.main.App.SelectDbEngineBin(dbEngineBinDir);
        } else if (dbEngineKind) {
          await window.go.main.App.SelectDbEngine(dbEngineKind);
        }
        await window.go.main.App.VerifyPostgresPassword(pgUser, pgPass);
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

  const uninstallLead = isLinux
    ? (
      <>
        Будут удалены папка установки, база <b>okidoci_db</b>, роли <b>okidoci_*</b>, <b>oki_*</b>, служба systemd <b>cyberstab-server</b> и ярлыки .desktop.
        Сам PostgreSQL не удаляется.
      </>
    )
    : (
      <>
        Будут удалены папка установки, база <b>okidoci_db</b>, роли <b>okidoci_*</b>, <b>oki_*</b>, задачи планировщика и ярлыки.
        Сам PostgreSQL не удаляется.
      </>
    );

  const skipDbLabel = isLinux
    ? "Не трогать базу данных (только файлы и службы Linux)"
    : "Не трогать базу данных (только файлы и Windows)";

  const canDelete =
    !busy &&
    (skipDB || (dbEngineBinDir.trim().length > 0 && pgUser.trim().length > 0 && pgPass.trim().length > 0));

  return (
    <div className="app">
      {showResetModal && (
        <div className="modalOverlay">
          <div className="modal">
            <div className="cardTitle">Мастер удаления</div>
            <div className="hint">
              Новый пароль для пользователя <b>{pgUser.trim() || "—"}</b>.
              <br />
              Сброс выполняется через локальный доступ СУБД (нужны права root / администратора).
            </div>
            <div className="row" style={{ marginTop: 12 }}>
              <input className="input" type="password" value={newPgPass} onChange={(e) => setNewPgPass(e.target.value)} placeholder="Новый пароль" disabled={resetBusy} />
            </div>
            <div className="row" style={{ marginTop: 10 }}>
              <input className="input" type="password" value={newPgPass2} onChange={(e) => setNewPgPass2(e.target.value)} placeholder="Повторите пароль" disabled={resetBusy} />
            </div>
            {resetError && (
              <div className="hint" style={{ marginTop: 10, color: "var(--error)" }}>
                {resetError}
              </div>
            )}
            <div className="wizardFooter">
              <button className={cls("btn", resetBusy && "disabled")} onClick={() => { setShowResetModal(false); setResetError(""); setNewPgPass(""); setNewPgPass2(""); }} disabled={resetBusy}>Закрыть</button>
              <button className={cls("btnPrimary", resetBusy && "disabled")} onClick={onResetPassword} disabled={resetBusy}>
                {resetBusy ? "Сменяю…" : "Сменить пароль"}
              </button>
            </div>
          </div>
        </div>
      )}

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
            <p className="wizardOs">ОС: {osLabel(info?.os)}</p>
          </div>

          <h2 className="wizardSectionTitle">Что удалить</h2>
          <div className="wizardBody">
            <p className="wizardHint uninstallLead">{uninstallLead}</p>

            <h3 className="uninstallSectionTitle">Папка установки</h3>
            {isLinux ? (
              <p className="wizardHint">На Linux папка фиксированная: /opt/cyberstab</p>
            ) : (
              <p className="wizardHint">На Windows можно указать папку установки.</p>
            )}
            <div className="pathRow uninstallPathRow">
              <input
                className="input uninstallInput"
                value={installDir}
                onChange={(e) => setInstallDir(e.target.value)}
                disabled={busy || isLinux}
                placeholder={isLinux ? "/opt/cyberstab" : "C:\\Program Files\\Cyberstab"}
              />
            </div>

            <h3 className="uninstallSectionTitle">PostgreSQL</h3>
            <label className={cls("radioRow uninstallToggle", skipDB && "radioRowOn", busy && "disabled")}>
              <input type="checkbox" checked={skipDB} onChange={(e) => setSkipDB(e.target.checked)} disabled={busy} />
              <span className="radioRowDot" />
              <span>{skipDbLabel}</span>
            </label>

            {!skipDB && (
              <>
                {dbEngines.length > 1 ? (
                  <div className="radioList" style={{ marginTop: 10, marginBottom: 10 }}>
                    {dbEngines.map((e) => (
                      <RadioRow
                        key={`${e.kind}:${e.binDir}`}
                        name="uninstallDbEngine"
                        label={e.label || dbEngineLabel(e.kind)}
                        checked={dbEngineBinDir === e.binDir}
                        onChange={async () => {
                          setDbEngineKind(e.kind);
                          setDbEngineBinDir(e.binDir);
                          setErrorText("");
                          await window.go.main.App.SelectDbEngineBin(e.binDir);
                        }}
                        disabled={busy}
                      />
                    ))}
                  </div>
                ) : dbEngines.length === 1 ? (
                  <p className="wizardHint" style={{ margin: "10px 0" }}>
                    Найдена СУБД: <b>{dbEngineLabel(dbEngineKind || dbEngines[0]?.kind, dbEngines)}</b>.
                  </p>
                ) : null}
                <p className="wizardHint">
                  Укажите пользователя СУБД с правами суперпользователя — он нужен для удаления базы и ролей.
                </p>
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
                    {info?.postgresInstalled && (
                      <button
                        type="button"
                        className={cls("btnLink", busy && "disabled")}
                        onClick={() => {
                          setShowResetModal(true);
                          setResetError("");
                          setNewPgPass("");
                          setNewPgPass2("");
                        }}
                        disabled={busy || pgUser.trim().length === 0}
                        title={pgUser.trim().length === 0 ? "Сначала укажите пользователя PostgreSQL" : undefined}
                      >
                        Забыли пароль?
                      </button>
                    )}
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
                        autoComplete="current-password"
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
                    {dbEngines.length >= 1 && (
                      <button
                        type="button"
                        className={cls("btnLink", busy && "disabled")}
                        onClick={onPickDbDir}
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
            <button className={cls("btnDanger", busy && "disabled")} onClick={onUninstall} disabled={!canDelete}>
              {busy ? "Удаление…" : "Удалить"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
