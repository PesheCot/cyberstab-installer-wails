import React, { useEffect, useMemo, useState } from "react";

type StepInfo = {
  Index: number;
  Description: string;
  Status: string;
  IsActive: boolean;
  IsDone: boolean;
  Error?: string | null;
};

type AppInfo = {
  os: string;
  needAdmin: boolean;
  postgresInstalled: boolean;
};

declare global {
  interface Window {
    go: any;
    runtime: any;
  }
}

function cls(...parts: Array<string | false | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export const App: React.FC = () => {
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [stage, setStage] = useState<0 | 1 | 2 | 3 | 4 | 5>(0);
  const [installServer, setInstallServer] = useState(true);
  const [installClients, setInstallClients] = useState(true);
  const [installDB, setInstallDB] = useState(false);
  const [sourceRoot, setSourceRoot] = useState("");
  const [installDir, setInstallDir] = useState("");
  const [pgPass, setPgPass] = useState("");
  const [running, setRunning] = useState(false);
  const [steps, setSteps] = useState<StepInfo[]>([]);
  const [errorText, setErrorText] = useState<string>("");
  const [pgVerified, setPgVerified] = useState<boolean>(false);
  const [pgVerifyError, setPgVerifyError] = useState<string>("");
  const [pgVerifying, setPgVerifying] = useState<boolean>(false);
  const [showResetModal, setShowResetModal] = useState(false);
  const [newPgPass, setNewPgPass] = useState("");
  const [newPgPass2, setNewPgPass2] = useState("");
  const [resetBusy, setResetBusy] = useState(false);
  const [resetError, setResetError] = useState<string>("");

  // Install dir conflict (Windows).
  const [showInstallDirModal, setShowInstallDirModal] = useState(false);
  const [pendingStartOpts, setPendingStartOpts] = useState<any>(null);

  // USB check state (stage 0)
  const [usbError, setUsbError] = useState<string>("");
  const [usbChecking, setUsbChecking] = useState<boolean>(false);

  // PostgreSQL installation state (stage 1)
  const [pgInstalled, setPgInstalled] = useState<boolean>(true);
  const [pgInstallerFound, setPgInstallerFound] = useState<boolean>(false);
  const [pgInstallerPath, setPgInstallerPath] = useState<string>("");
  const [pgInstalling, setPgInstalling] = useState<boolean>(false);
  const [pgInstallError, setPgInstallError] = useState<string>("");

  // DB step state (stage 4)
  const [dbExists, setDbExists] = useState<boolean>(false);
  const [dbChecking, setDbChecking] = useState<boolean>(false);
  const [dbAction, setDbAction] = useState<"skip" | "new" | "restore">("new");
  const [restoreSqlPath, setRestoreSqlPath] = useState<string>("");

  // Installation completion state.
  const [installDone, setInstallDone] = useState<boolean>(false);
  const [openCyberstab, setOpenCyberstab] = useState<boolean>(true);

  // File copy progress bar (engine step 3).
  const [deployPct, setDeployPct] = useState<number>(0);
  const [deployStatus, setDeployStatus] = useState<string>("");

  // DB init progress bar (engine step 4).
  const [initPct, setInitPct] = useState<number>(0);
  const [initStatus, setInitStatus] = useState<string>("");

  const needsPG = installServer || installDB;

  // When any background operation is running, lock the UI (except Cancel).
  const uiLocked =
    running ||
    usbChecking ||
    pgVerifying ||
    pgInstalling ||
    dbChecking ||
    resetBusy;

  const readyToStart = useMemo(() => {
    if (!installServer && !installClients && !installDB) return false;
    if (needsPG && pgPass.trim().length === 0) return false;
    return true;
  }, [installServer, installClients, installDB, needsPG, pgPass]);

  useEffect(() => {
    (async () => {
      const i: AppInfo = await window.go.main.App.GetAppInfo();
      setInfo(i);
      if (i.os === "windows") setInstallDir("C:\\Program Files\\Cyberstab");

      // Auto-detect distro on start (USB/root scan) and prefill if found.
      try {
        const p = await window.go.main.App.AutoDetectSourceRoot(installServer, installClients);
        if (p && !sourceRoot) setSourceRoot(p);
      } catch {
        // ignore
      }
    })();

    window.runtime.EventsOn("install:progress", (payload: any) => {
      if (payload?.steps) setSteps(payload.steps);
    });
    window.runtime.EventsOn("install:step", (payload: any) => {
      if (payload?.steps) setSteps(payload.steps);
    });
    window.runtime.EventsOn("install:done", () => {
      setRunning(false);
      setInstallDone(true);
    });
    window.runtime.EventsOn("deploy:progress", (payload: any) => {
      const p = payload?.percent ?? payload?.percentage;
      if (p !== undefined) setDeployPct(p);
      if (payload?.status !== undefined) setDeployStatus(payload.status);
    });
    window.runtime.EventsOn("init:progress", (payload: any) => {
      const p = payload?.percent ?? payload?.percentage;
      if (p !== undefined) setInitPct(p);
      if (payload?.status !== undefined) setInitStatus(payload.status);
    });
  }, []);

  // Check PostgreSQL installation when entering stage 1.
  useEffect(() => {
    if (stage !== 1) return;
    (async () => {
      try {
        const result = await window.go.main.App.CheckPgInstalled();
        setPgInstalled(result.installed);
        setPgInstallerFound(result.installerFound);
        setPgInstallerPath(result.installerPath);
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
      } catch {
        // ignore
      }
    })();
  }, [stage]);

  // Re-try auto-detect when component selection changes (if user hasn't typed a path).
  useEffect(() => {
    (async () => {
      if (sourceRoot.trim().length > 0) return;
      try {
        const p = await window.go.main.App.AutoDetectSourceRoot(installServer || installDB, installClients);
        if (p) setSourceRoot(p);
      } catch {
        // ignore
      }
    })();
  }, [installServer, installClients, installDB]);

  // Auto-detect USB distro when entering step 4 (distro selection).
  useEffect(() => {
    if (stage !== 3) return;
    (async () => {
      try {
        const p = await window.go.main.App.AutoDetectSourceRoot(installServer || installDB, installClients);
        if (p) setSourceRoot(p);
      } catch {
        // ignore
      }
    })();
  }, [stage]);

  // Check okidoci_db when entering DB step (stage 4).
  useEffect(() => {
    if (stage !== 4) return;
    if (!needsPG) return;
    setDbChecking(true);
    (async () => {
      try {
        const result = await window.go.main.App.CheckOkidociDB(pgPass);
        setDbExists(result.exists);
        // По умолчанию — пересоздание с дампом; «оставить БД» нужно выбрать явно.
        setDbAction("new");
      } catch {
        setDbExists(false);
        setDbAction("new");
      } finally {
        setDbChecking(false);
      }
    })();
  }, [stage]);

  const onPickFolder = async () => {
    const p = await window.go.main.App.PickFolder();
    if (p) setSourceRoot(p);
  };

  const onPickInstallDir = async () => {
    const p = await window.go.main.App.PickFolder();
    if (p) setInstallDir(p);
  };

  const onInstallPgFromUsb = async () => {
    setPgInstalling(true);
    setPgInstallError("");
    try {
      await window.go.main.App.InstallPostgresFromUsb();
      const result = await window.go.main.App.CheckPgInstalled();
      setPgInstalled(result.installed);
      setPgInstallerFound(result.installerFound);
      setPgInstallerPath(result.installerPath);
      if (!result.installed) {
        setPgInstallError("PostgreSQL не обнаружен после установки. Попробуйте снова или укажите путь вручную.");
      } else {
        setPgInstallError("");
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
      }
    } catch (e: any) {
      setPgInstallError(String(e?.message || e));
    } finally {
      setPgInstalling(false);
    }
  };

  const onPickPgDir = async () => {
    setPgInstallError("");
    try {
      const dir = await window.go.main.App.PickPgDir();
      if (dir) {
        setPgInstalled(true);
        setPgInstallError("");
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
      }
    } catch (e: any) {
      setPgInstallError(String(e?.message || e));
    }
  };

  const onPickSqlFile = async () => {
    try {
      const p = await window.go.main.App.PickSqlFile();
      if (p) setRestoreSqlPath(p);
    } catch {
      // ignore
    }
  };

  const onStart = async () => {
    setErrorText("");
    const pending = {
      installServer,
      installClients,
      installDB,
      sourceRoot,
      installDir,
      postgresPassword: pgPass,
      dbAction: needsPG ? dbAction : "",
      restoreSqlPath: dbAction === "restore" ? restoreSqlPath : "",
    };

    try {
      // Windows: if install directory exists, ask user what to do.
      const conflict = await window.go.main.App.CheckInstallDirConflict(installDir);
      if (conflict?.exists) {
        setPendingStartOpts(pending);
        setShowInstallDirModal(true);
        return;
      }

      setRunning(true);
      await window.go.main.App.StartInstall({ ...pending, reinstallExisting: true });
    } catch (e: any) {
      setRunning(false);
      setErrorText(String(e?.message || e));
    }
  };

  const startInstallFromPending = async (reinstallExisting: boolean) => {
    if (!pendingStartOpts) return;
    const pending = pendingStartOpts;
    setShowInstallDirModal(false);
    setPendingStartOpts(null);

    setRunning(true);
    setErrorText("");
    try {
      await window.go.main.App.StartInstall({ ...pending, reinstallExisting });
    } catch (e: any) {
      setRunning(false);
      setErrorText(String(e?.message || e));
    }
  };

  const canNext = useMemo(() => {
    if (stage === 0) {
      if (usbChecking) return false;
      return installServer || installClients || installDB;
    }
    if (stage === 1) {
      if (!needsPG) return true;
      if (!pgInstalled) return false;
      if (pgInstalling) return false;
      return pgPass.trim().length > 0;
    }
    if (stage === 2) {
      if (info?.os !== "windows") return true;
      return installDir.trim().length > 0;
    }
    if (stage === 3) return sourceRoot.trim().length > 0;
    if (stage === 4) {
      if (!needsPG) return true;
      if (dbChecking) return false;
      if (dbAction === "restore" && restoreSqlPath.trim().length === 0) return false;
      return true;
    }
    return true;
  }, [stage, installServer, installClients, installDB, needsPG, pgPass, info?.os, installDir, sourceRoot, dbAction, restoreSqlPath, dbChecking, usbChecking, pgInstalled, pgInstalling]);

  const onCancel = () => {
    // If installation is done and user wants to open Cyberstab, do it before quitting
    if (installDone && openCyberstab) {
      launchCyberstab();
    }
    window.runtime.Quit();
  };

  const launchCyberstab = () => {
    try {
      window.go.main.App.LaunchClient();
    } catch {
      // Ignore launch errors
    }
  };

  const onNext = async () => {
    setPgVerifyError("");
    setUsbError("");
    if (uiLocked) return;

    // Stage 0: mandatory USB check before proceeding.
    if (stage === 0) {
      setUsbChecking(true);
      try {
        const p = await window.go.main.App.AutoDetectSourceRoot(installServer || installDB, installClients);
        if (p) {
          setSourceRoot(p);
          setUsbError("");
          setUsbChecking(false);
          setStage(1);
        } else {
          setUsbError("Флешка с дистрибутивом не найдена. Вставьте USB-накопитель с файлами CyberstabServer*/CyberstabClient* и попробуйте снова.");
          setUsbChecking(false);
        }
      } catch {
        setUsbError("Не удалось проверить USB-накопители.");
        setUsbChecking(false);
      }
      return;
    }

    // Stage 1: verify password when PG is installed.
    if (stage === 1 && needsPG && pgInstalled) {
      setPgVerifying(true);
      try {
        await window.go.main.App.VerifyPostgresPassword(pgPass);
        setPgVerified(true);
        setPgVerifyError("");
        setPgVerifying(false);
        setStage((s) => (s < 5 ? ((s + 1) as any) : s));
      } catch (_e: any) {
        setPgVerified(false);
        setPgVerifying(false);
        const msg = String(_e?.message || _e || "").toLowerCase();
        if (msg.includes("timeout") || msg.includes("время ожидания")) {
          setPgVerifyError(String(_e?.message || _e));
        } else if (msg.includes("not found") || msg.includes("не найден") || msg.includes("не удается") || msg.includes("не удаётся")) {
          setPgVerifyError("PostgreSQL не найден. Убедитесь, что PostgreSQL установлен.");
        } else {
          setPgVerifyError("Пароль неверный. Проверьте пароль и попробуйте снова, или нажмите «Забыли пароль?»");
        }
        return;
      }
    } else {
      setStage((s) => (s < 5 ? ((s + 1) as any) : s));
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
    setResetBusy(true);
    try {
      await window.go.main.App.ResetPostgresPassword(newPgPass);
      // SUCCESS – replace password in the main form and close modal
      setPgPass(newPgPass);
      setPgVerified(true);
      setPgVerifyError("");
      setShowResetModal(false);
      setNewPgPass("");
      setNewPgPass2("");
    } catch (e: any) {
      setResetError(String(e?.message || e));
    } finally {
      setResetBusy(false);
    }
  };

  // Determine if we're in installation progress (running but not done)
  const isInstalling = running && !installDone;

  return (
    <div className="app">
      {showResetModal && (
        <div className="modalOverlay">
          <div className="modal">
            <div className="cardTitle">Смена пароля PostgreSQL</div>
            <div className="hint">Введите новый пароль для пользователя <b>postgres</b>.</div>
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

      {showInstallDirModal && (
        <div className="modalOverlay">
          <div className="modal">
            <div className="cardTitle">Папка уже существует</div>
            <div className="hint">
              Папка <b>{installDir}</b> уже существует. Выберите режим установки:
            </div>
            <div className="hint" style={{ marginTop: 10 }}>
              При <b>переустановке</b> файл <b>server.properties</b> будет сохранён как <b>server.properties.back</b> и затем подставлен обратно.
            </div>
            <div className="wizardFooter" style={{ marginTop: 10 }}>
              <button
                className={cls("btn", running && "disabled")}
                onClick={() => {
                  setShowInstallDirModal(false);
                  setPendingStartOpts(null);
                }}
                disabled={running}
              >
                Отмена
              </button>
              <button className={cls("btn", running && "disabled")} onClick={() => startInstallFromPending(false)} disabled={running}>
                Оставить
              </button>
              <button className={cls("btnPrimary", running && "disabled")} onClick={() => startInstallFromPending(true)} disabled={running}>
                Переустановить
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="topBar">
        <div className="topBarInner" style={{ ["--wails-draggable" as any]: "drag" }}>
          <div className="topLeft">
            <div className="topLogo">C</div>
            <div className="topTitle">Cyberstab Installer</div>
          </div>
          <div className="topWinButtons" style={{ ["--wails-draggable" as any]: "no-drag" }}>
            <button className="winBtn" onClick={() => window.runtime.WindowMinimise()} aria-label="Minimise">—</button>
            <button className="winBtn" onClick={() => window.runtime.WindowToggleMaximise()} aria-label="Maximise">□</button>
            {/* Hide close button on completion screen */}
            {!installDone && (
              <button className="winBtn winClose" onClick={() => onCancel()} aria-label="Close">×</button>
            )}
          </div>
        </div>
      </div>
      <div className="content">
      <div className={cls("grid", (running || installDone) && "dual")}>
        <div className="card">
          <div className="cardTitle">Мастер установки</div>
          <div className="hint">
            Шаг {stage + 1} из 6 · ОС: {info?.os === "windows" ? "Windows" : info?.os === "linux" ? "Linux" : (info?.os || "—")}
          </div>

          {stage === 0 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>Что установить</div>
              <div className="row" style={{ flexWrap: "wrap" }}>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={installServer}
                    onChange={(e) => {
                      setInstallServer(e.target.checked);
                      if (e.target.checked) setInstallDB(false);
                    }}
                    disabled={uiLocked || installDB}
                  />
                  <span>Сервер</span>
                </label>
                <label className="check">
                  <input type="checkbox" checked={installClients} onChange={(e) => setInstallClients(e.target.checked)} disabled={uiLocked} />
                  <span>Клиент</span>
                </label>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={installDB}
                    onChange={(e) => {
                      setInstallDB(e.target.checked);
                      if (e.target.checked) setInstallServer(false);
                    }}
                    disabled={uiLocked || installServer}
                  />
                  <span>Только БД</span>
                </label>
              </div>
              <div className="hint" style={{ marginTop: 8 }}>
                {installDB
                  ? "Будут скопированы только serverconsole и dbupdater для инициализации/восстановления базы данных."
                  : info?.os === "windows"
                    ? "Клиент будет выбран автоматически по разрядности системы (Windows64 на 64-bit)."
                    : "Клиент будет выбран автоматически (Linux64 на 64-bit)."}
              </div>
              {usbChecking && (
                <div className="hint" style={{ marginTop: 10 }}>Поиск USB-накопителя с дистрибутивом…</div>
              )}
              {usbError && (
                <div className="hint" style={{ marginTop: 10, color: "var(--error)" }}>
                  {usbError}
                </div>
              )}
            </>
          )}

          {stage === 1 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>PostgreSQL</div>
              {!needsPG ? (
                <div className="hint">Сервер/БД не выбран — PostgreSQL будет пропущен.</div>
              ) : !pgInstalled ? (
                <>
                  <div className="hint" style={{ color: "var(--error)" }}>
                    PostgreSQL не найден по стандартному пути (C:\Program Files\PostgreSQL\...\bin).
                  </div>
                  <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 10 }}>
                    {pgInstallerFound && (
                      <button
                        className={cls("btnPrimary", pgInstalling && "disabled")}
                        onClick={onInstallPgFromUsb}
                        disabled={pgInstalling}
                      >
                        {pgInstalling ? "Установка PostgreSQL…" : `Установить PostgreSQL с флешки`}
                      </button>
                    )}
                    {!pgInstallerFound && (
                      <div className="hint">Установщик PostgreSQL (postgresql*.exe) не найден на USB-накопителе.</div>
                    )}
                    <button className="btn" onClick={onPickPgDir} disabled={pgInstalling}>
                      Указать путь до PostgreSQL вручную…
                    </button>
                  </div>
                  {pgInstallError && (
                    <div className="hint" style={{ marginTop: 10, color: "var(--error)" }}>
                      {pgInstallError}
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="hint">
                    Пароль postgres нужен для настройки и инициализации БД.
                  </div>
                  <div className="row" style={{ gap: "8px" }}>
                    <input
                      className="input"
                      type="password"
                      value={pgPass}
                      onChange={(e) => {
                        setPgPass(e.target.value);
                        setPgVerified(false);
                        setPgVerifyError("");
                      }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          onNext();
                        }
                      }}
                      placeholder="Пароль postgres"
                      disabled={uiLocked}
                      style={pgVerified ? { borderColor: "var(--success)" } : pgVerifyError ? { borderColor: "var(--error)" } : {}}
                    />
                  </div>
                  {pgVerified && (
                    <div className="hint" style={{ marginTop: 6, color: "var(--success)" }}>
                      ✓ Пароль подтверждён
                    </div>
                  )}
                  {pgVerifyError && (
                    <div className="hint" style={{ marginTop: 6, color: "var(--error)" }}>
                      ✗ {pgVerifyError}
                    </div>
                  )}
                  {info?.postgresInstalled && (
                    <div className="row" style={{ marginTop: 10, justifyContent: "flex-start" }}>
                      <button className="btn" onClick={() => {
                        setShowResetModal(true);
                        setResetError("");
                        setNewPgPass("");
                        setNewPgPass2("");
                      }} disabled={uiLocked}>
                        Забыли пароль?
                      </button>
                    </div>
                  )}
                </>
              )}
            </>
          )}

          {stage === 2 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>Папка установки</div>
              {info?.os === "windows" ? (
                <>
                  <div className="hint">На Windows можно выбрать папку установки.</div>
                  <div className="row">
                    <input className="input" value={installDir} onChange={(e) => setInstallDir(e.target.value)} placeholder="Например: C:\\Program Files" disabled={uiLocked} />
                    <button className="btn" onClick={onPickInstallDir} disabled={uiLocked}>Выбрать…</button>
                  </div>
                </>
              ) : (
                <div className="hint">На Linux папка фиксированная: /opt/cyberstab</div>
              )}
            </>
          )}

          {stage === 3 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>Дистрибутив</div>
              <div className="hint">Выберите родительскую папку, где лежат папки `CyberstabServer*` / `CyberstabClient*`.</div>
              <div className="row">
                <input className="input" value={sourceRoot} onChange={(e) => setSourceRoot(e.target.value)} placeholder="Папка-дистрибутив (USB или локальная)" disabled={uiLocked} />
                <button className="btn" onClick={onPickFolder} disabled={uiLocked}>Выбрать…</button>
              </div>
            </>
          )}

          {stage === 4 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>База данных</div>
              {!needsPG ? (
                <div className="hint">Сервер/БД не выбран — настройка БД пропущена.</div>
              ) : dbChecking ? (
                <div className="hint">Проверка базы данных okidoci_db…</div>
              ) : !dbExists ? (
                <>
                  <div className="hint">База данных <b>okidoci_db</b> не найдена. Выберите действие:</div>
                  <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 8 }}>
                    <label className="check">
                      <input type="radio" name="dbAction" checked={dbAction === "new"} onChange={() => setDbAction("new")} />
                      <span>Создать новую БД</span>
                    </label>
                    <label className="check">
                      <input type="radio" name="dbAction" checked={dbAction === "restore"} onChange={() => setDbAction("restore")} />
                      <span>Восстановить из бэкапа (.sql)</span>
                    </label>
                  </div>
                  {dbAction === "restore" && (
                    <div className="row" style={{ marginTop: 10 }}>
                      <input
                        className="input"
                        value={restoreSqlPath}
                        onChange={(e) => setRestoreSqlPath(e.target.value)}
                        placeholder="Путь к .sql файлу"
                        disabled={uiLocked}
                      />
                      <button className="btn" onClick={onPickSqlFile} disabled={uiLocked}>Выбрать…</button>
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="hint">База данных <b>okidoci_db</b> уже существует. Выберите действие:</div>
                  <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 8 }}>
                    <label className="check">
                      <input type="radio" name="dbAction" checked={dbAction === "skip"} onChange={() => setDbAction("skip")} />
                      <span>Оставить текущую БД (пропустить инициализацию)</span>
                    </label>
                    <label className="check">
                      <input type="radio" name="dbAction" checked={dbAction === "new"} onChange={() => setDbAction("new")} />
                      <span>Удалить + создать новую (бэкап будет сохранён)</span>
                    </label>
                    <label className="check">
                      <input type="radio" name="dbAction" checked={dbAction === "restore"} onChange={() => setDbAction("restore")} />
                      <span>Удалить + восстановить из бэкапа (бэкап будет сохранён)</span>
                    </label>
                  </div>
                  {dbAction === "restore" && (
                    <div className="row" style={{ marginTop: 10 }}>
                      <input
                        className="input"
                        value={restoreSqlPath}
                        onChange={(e) => setRestoreSqlPath(e.target.value)}
                        placeholder="Путь к .sql файлу"
                        disabled={uiLocked}
                      />
                      <button className="btn" onClick={onPickSqlFile} disabled={uiLocked}>Выбрать…</button>
                    </div>
                  )}
                </>
              )}
            </>
          )}

          {stage === 5 && (
            <>
              <div className="cardTitle" style={{ marginTop: 14 }}>
                Запуск
              </div>
              <div className="hint">Проверьте параметры и нажмите "Начать установку".</div>
              <div className="row" style={{ marginTop: 16 }}>
                <button className={cls("btnPrimary", (!readyToStart || uiLocked) && "disabled")} onClick={onStart} disabled={!readyToStart || uiLocked}>
                  {running ? "Устанавливаю…" : uiLocked ? "Подождите…" : "Начать установку"}
                </button>
              </div>
              <div className="hint" style={{ marginTop: 10 }}>
                {info?.needAdmin ? "Запустите установщик от администратора." : " "}
              </div>
            </>
          )}

          {stage !== 1 && errorText && !running && (
            <div className="hint" style={{ marginTop: 10, color: "var(--error)" }}>
              {errorText}
            </div>
          )}

        </div>

        {(running || installDone) && (
          <div className="card">
            <div className="cardTitle">Ход установки</div>
            <div className="steps">
              {steps.map((s) => (
                <div key={s.Index} className={cls("step", s.IsActive && "active", s.IsDone && "done", !!s.Error && "error")}>
                  <div className="stepIcon">
                    {s.Error ? "✗" : s.IsDone ? "✓" : s.IsActive ? "…" : "•"}
                  </div>
                  <div className="stepBody">
                    <div className="stepTitle">{s.Description}</div>
                    {s.Status && <div className="stepStatus">{s.Status}</div>}
                    {s.Error && (
                      <div className="stepStatus" style={{ color: "var(--error)" }}>
                        {s.Error}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {deployStatus && deployPct < 100 && (
              <div className="initProgressSection">
                <div className="initProgressLabel">
                  <span>Копирование файлов</span>
                  <span>{Math.round(deployPct)}%</span>
                </div>
                <div className="initProgressBar">
                  <div
                    className={cls("initProgressBarFill", deployPct >= 99 && "done")}
                    style={{ width: `${deployPct}%` }}
                  />
                </div>
                <div className="initProgressStatus">{deployStatus}</div>
              </div>
            )}

            {initStatus && initPct < 100 && (
              <div className="initProgressSection">
                <div className="initProgressLabel">
                  <span>Инициализация базы данных</span>
                  <span>{Math.round(initPct)}%</span>
                </div>
                <div className="initProgressBar">
                  <div
                    className={cls("initProgressBarFill", initPct >= 99 && "done")}
                    style={{ width: `${initPct}%` }}
                  />
                </div>
                <div className="initProgressStatus">{initStatus}</div>
                {initPct > 0 && initPct < 100 && (
                  <div className="initProgressHint">
                    Шкала ориентирована примерно на 2 часа; при более быстром завершении дойдёт до ~99% и перескочит на 100%.
                  </div>
                )}
              </div>
            )}

            {/* Completion screen */}
            {installDone && (
              <div className="initProgressSection" style={{ marginTop: 20 }}>
                <div style={{ textAlign: "center", marginBottom: 20 }}>
                  <div style={{ 
                    fontSize: 48, 
                    marginBottom: 12,
                    color: steps.some((s) => s.Error) ? "var(--error)" : "var(--success)"
                  }}>
                    {steps.some((s) => s.Error) ? "!" : "✓"}
                  </div>
                  <div className="cardTitle" style={{ fontSize: 20, marginBottom: 8 }}>
                    {steps.some((s) => s.Error) ? "Установка завершена с ошибками" : "Установка завершена"}
                  </div>
                  {steps.some((s) => s.Error) ? (
                    <div className="hint" style={{ color: "var(--error)" }}>
                      Проверьте детали в панели «Ход установки».
                    </div>
                  ) : (
                    <div className="hint" style={{ color: "var(--success)" }}>
                      Cyberstab успешно установлен!
                    </div>
                  )}
                </div>

                {installClients && (
                  <label className="check" style={{ marginBottom: 16, justifyContent: "center" }}>
                    <input 
                      type="checkbox" 
                      checked={openCyberstab} 
                      onChange={(e) => setOpenCyberstab(e.target.checked)} 
                    />
                    <span style={{ fontWeight: 600 }}>Открыть Киберстаб</span>
                  </label>
                )}
              </div>
            )}
          </div>
        )}
      </div>
      </div>

      {/* Footer buttons - show/hide based on state */}
      {!installDone && !isInstalling && (
        <div className="footerBar">
          <div className="footerInner">
            <button
              className={cls("btn", (uiLocked || stage === 0) && "disabled")}
              onClick={() => {
                if (uiLocked || stage === 0) return;
                setStage((s) => (s > 0 ? ((s - 1) as any) : s));
              }}
              disabled={uiLocked || stage === 0}
            >
              Назад
            </button>
            <button
              className={cls("btnPrimary", (!canNext || uiLocked || stage === 5) && "disabled")}
              onClick={onNext}
              disabled={uiLocked || !canNext || stage === 5}
              style={stage === 4 ? { minWidth: 200 } : {}}
            >
              {usbChecking ? "Поиск флешки…" : pgVerifying ? "Проверяю…" : stage === 4 ? "Начать установку" : "Далее"}
            </button>
            <button className="btn" onClick={onCancel}>
              Отмена
            </button>
          </div>
        </div>
      )}

      {/* During installation - only Cancel button */}
      {isInstalling && (
        <div className="footerBar">
          <div className="footerInner" style={{ justifyContent: "center" }}>
            <button className="btn" onClick={onCancel} style={{ minWidth: 140 }}>
              Отмена
            </button>
          </div>
        </div>
      )}

      {/* After installation - no buttons, just close via window controls */}
      {installDone && (
        <div className="footerBar">
          <div className="footerInner" style={{ justifyContent: "center" }}>
            <button className="btnPrimary" onClick={onCancel} style={{ minWidth: 140 }}>
              {openCyberstab ? "Закрыть и открыть" : "Закрыть"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
