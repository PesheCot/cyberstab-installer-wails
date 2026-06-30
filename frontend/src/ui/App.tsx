import React, { useEffect, useMemo, useRef, useState } from "react";
import cyberstabLogo from "../assets/cyberstab-logo.svg";

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

type DbEngineKind = "postgresql" | "postgrespro" | "jatoba";

type DbEngineInfo = {
  kind: DbEngineKind;
  label: string;
  binDir: string;
  version?: string;
  isManual?: boolean;
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

const STAGE_SECTIONS = [
  "Что установить",
  "СУБД",
  "Папка установки",
  "Дистрибутив",
  "База данных (БД)",
  "Запуск",
] as const;

const TOTAL_VISIBLE_STEPS = 5;

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

function buildInstallStepsPreview(
  reinstallExisting: boolean,
  dbAction: "skip" | "new" | "restore",
  hasDbComponent: boolean,
): StepInfo[] {
  const steps: StepInfo[] = [
    { Index: 1, Description: "Проверка параметров", Status: "", IsActive: false, IsDone: false },
    { Index: 2, Description: "Подготовка папки установки", Status: "", IsActive: false, IsDone: false },
  ];
  let idx = 3;
  if (reinstallExisting) {
    steps.push({ Index: idx++, Description: "Копирование файлов", Status: "", IsActive: false, IsDone: false });
  }
  if (hasDbComponent && dbAction !== "skip") {
    steps.push({ Index: idx++, Description: "Инициализация базы данных", Status: "", IsActive: false, IsDone: false });
  }
  steps.push({ Index: idx, Description: "Завершение", Status: "", IsActive: false, IsDone: false });
  return steps;
}

const OptionChip: React.FC<{
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}> = ({ label, checked, onChange, disabled }) => (
  <button
    type="button"
    className={cls("optionChip", checked && "optionChipOn", disabled && "disabled")}
    onClick={() => !disabled && onChange(!checked)}
    disabled={disabled}
  >
    <span className="optionChipMark" aria-hidden>{checked ? "✓" : ""}</span>
    <span>{label}</span>
  </button>
);

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

export const App: React.FC = () => {
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [stage, setStage] = useState<0 | 1 | 2 | 3 | 4 | 5>(0);
  const [installServer, setInstallServer] = useState(true);
  const [installClients, setInstallClients] = useState(true);
  const [installDB, setInstallDB] = useState(false);
  const [sourceRoot, setSourceRoot] = useState("");
  const [installDir, setInstallDir] = useState("");
  const [pgUser, setPgUser] = useState("postgres");
  const [pgPass, setPgPass] = useState("");
  const [showPgPass, setShowPgPass] = useState(false);
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

  // DB engine state (stage 1)
  const [dbEngines, setDbEngines] = useState<DbEngineInfo[]>([]);
  const [dbEngineKind, setDbEngineKind] = useState<DbEngineKind | "">("");
  const [dbEngineBinDir, setDbEngineBinDir] = useState("");
  const [dbInstalled, setDbInstalled] = useState<boolean>(true);
  const [pgInstallerFound, setPgInstallerFound] = useState<boolean>(false);
  const [pgInstallerPath, setPgInstallerPath] = useState<string>("");
  const [pgPackageOptions, setPgPackageOptions] = useState<{ path: string; label: string; version: string }[]>([]);
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
  const clientLaunchedRef = useRef(false);
  const [closing, setClosing] = useState(false);

  // File copy progress bar (engine step 3).
  const [deployPct, setDeployPct] = useState<number>(0);
  const [deployStatus, setDeployStatus] = useState<string>("");

  // DB init progress bar (engine step 4).
  const [initPct, setInitPct] = useState<number>(0);
  const [initStatus, setInitStatus] = useState<string>("");
  const [desktopPreviewDone, setDesktopPreviewDone] = useState(false);

  const needsPG = installServer || installDB;
  const urlPreviewDone = typeof window !== "undefined" && new URLSearchParams(window.location.search).get("preview") === "install-done";
  const previewDone = urlPreviewDone || desktopPreviewDone;

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
    if (needsPG && (pgUser.trim().length === 0 || pgPass.trim().length === 0)) return false;
    return true;
  }, [installServer, installClients, installDB, needsPG, pgUser, pgPass]);

  useEffect(() => {
    (async () => {
      if (urlPreviewDone) return;
      try {
        if (await window.go.main.App.PreviewInstallDone()) {
          setDesktopPreviewDone(true);
          return;
        }
      } catch {
        // ignore
      }

      const i: AppInfo = await window.go.main.App.GetAppInfo();
      setInfo(i);
      if (i.os === "windows") setInstallDir("C:\\Program Files\\Cyberstab");
      if (i.os === "linux") setInstallDir("/opt/cyberstab");

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

  // Check DB engine availability when entering stage 1.
  useEffect(() => {
    if (stage !== 1) return;
    (async () => {
      try {
        const result = await window.go.main.App.CheckDbInstalled();
        const engines = (result.engines || []) as DbEngineInfo[];
        setDbEngines(engines);
        setDbInstalled(result.installed);
        setPgInstallerFound(result.installerFound);
        setPgInstallerPath(result.installerPath);
        if (!result.installed) {
          const i: AppInfo = await window.go.main.App.GetAppInfo();
          if (i.os === "linux" && result.installerFound) {
            const pkgs = await window.go.main.App.ListPostgresInstallers(sourceRoot || "");
            setPgPackageOptions((pkgs || []) as { path: string; label: string; version: string }[]);
          } else {
            setPgPackageOptions([]);
          }
        } else {
          setPgPackageOptions([]);
        }
        if (engines.length === 1) {
          const e = engines[0];
          setDbEngineKind(e.kind);
          setDbEngineBinDir(e.binDir);
          await window.go.main.App.SelectDbEngineBin(e.binDir);
        } else if (engines.length >= 2) {
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
        const i: AppInfo = await window.go.main.App.GetAppInfo();
        setInfo(i);
      } catch {
        // ignore
      }
    })();
  }, [stage, sourceRoot]);

  // Re-try auto-detect when component selection changes (if user hasn't typed a path).
  useEffect(() => {
    if (previewDone) return;

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
        if (dbEngineBinDir) {
          await window.go.main.App.SelectDbEngineBin(dbEngineBinDir);
        } else if (dbEngineKind) {
          await window.go.main.App.SelectDbEngine(dbEngineKind);
        }
        const result = await window.go.main.App.CheckOkidociDB(pgUser, pgPass);
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
  }, [stage, dbEngineKind]);

  const onPickFolder = async () => {
    const p = await window.go.main.App.PickFolder();
    if (p) setSourceRoot(p);
  };

  const onPickInstallDir = async () => {
    const p = await window.go.main.App.PickFolder();
    if (p) setInstallDir(p);
  };

  const refreshDbAfterPgInstall = async () => {
    const result = await window.go.main.App.CheckDbInstalled();
    const engines = (result.engines || []) as DbEngineInfo[];
    setDbEngines(engines);
    setDbInstalled(result.installed);
    setPgInstallerFound(result.installerFound);
    setPgInstallerPath(result.installerPath);
    if (engines.length > 0) {
      const preferred = pickPreferredEngine(engines);
      if (preferred) {
        setDbEngineKind(preferred.kind);
        setDbEngineBinDir(preferred.binDir);
        await window.go.main.App.SelectDbEngineBin(preferred.binDir);
      }
    }
    if (!result.installed) {
      setPgInstallError("СУБД не обнаружена после установки. Попробуйте снова или укажите путь вручную.");
    } else {
      setPgInstallError("");
      const i: AppInfo = await window.go.main.App.GetAppInfo();
      setInfo(i);
    }
  };

  const onInstallPgFromUsb = async () => {
    setPgInstalling(true);
    setPgInstallError("");
    try {
      await window.go.main.App.InstallPostgresFromUsb();
      await refreshDbAfterPgInstall();
    } catch (e: any) {
      setPgInstallError(String(e?.message || e));
    } finally {
      setPgInstalling(false);
    }
  };

  const onInstallPgPackage = async (version: string) => {
    setPgInstalling(true);
    setPgInstallError("");
    try {
      await window.go.main.App.InstallPostgresInstaller(version);
      await refreshDbAfterPgInstall();
    } catch (e: any) {
      setPgInstallError(String(e?.message || e));
    } finally {
      setPgInstalling(false);
    }
  };

  const onPickDbDir = async () => {
    setPgInstallError("");
    try {
      const dir = await window.go.main.App.PickDbDir();
      if (dir) {
        const result = await window.go.main.App.CheckDbInstalled();
        const engines = (result.engines || []) as DbEngineInfo[];
        setDbEngines(engines);
        setDbInstalled(result.installed);
        const selected = engines.find((e) => e.binDir.toLowerCase().includes(dir.toLowerCase())) ?? pickPreferredEngine(engines);
        if (selected) {
          setDbEngineKind(selected.kind);
          setDbEngineBinDir(selected.binDir);
          await window.go.main.App.SelectDbEngineBin(selected.binDir);
        }
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

  const beginInstall = (reinstallExisting: boolean, pending: {
    installServer: boolean;
    installClients: boolean;
    installDB: boolean;
    sourceRoot: string;
    installDir: string;
    dbEngine: string;
    postgresUser: string;
    postgresPassword: string;
    dbAction: string;
    restoreSqlPath: string;
  }) => {
    const hasDbComponent = pending.installServer || pending.installDB;
    const action = (pending.dbAction || "new") as "skip" | "new" | "restore";
    setDeployPct(0);
    setDeployStatus("");
    setInitPct(0);
    setInitStatus("");
    setInstallDone(false);
    setSteps(buildInstallStepsPreview(reinstallExisting, action, hasDbComponent));
    setRunning(true);
    setErrorText("");
  };

  const onStart = async () => {
    setErrorText("");
    const pending = {
      installServer,
      installClients,
      installDB,
      sourceRoot,
      installDir,
      dbEngine: dbEngineKind,
      postgresUser: pgUser,
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

      beginInstall(true, pending);
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

    beginInstall(reinstallExisting, pending);
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
      if (!dbInstalled) return false;
      if (pgInstalling) return false;
      if (dbEngines.length > 1 && !dbEngineKind) return false;
      return pgUser.trim().length > 0 && pgPass.trim().length > 0;
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
  }, [stage, installServer, installClients, installDB, needsPG, pgUser, pgPass, info?.os, installDir, sourceRoot, dbAction, restoreSqlPath, dbChecking, usbChecking, dbInstalled, pgInstalling, dbEngines.length, dbEngineKind]);

  const onCancel = async () => {
    if (closing) return;
    setClosing(true);

    // If installation is done and user wants to open Cyberstab, do it BEFORE quitting.
    // IMPORTANT: Wails calls are async; if we Quit immediately, the call may not reach backend.
    if (installDone && openCyberstab && !clientLaunchedRef.current) {
      try {
        clientLaunchedRef.current = true;
        await window.go.main.App.LaunchClient(installDir);
      } catch {
        // ignore
      }
      // Give the OS a moment to spawn the process before quitting.
      window.setTimeout(() => window.runtime.Quit(), 400);
      return;
    }

    if (running) {
      try {
        await window.go.main.App.CancelInstall();
      } catch {
        // The app is closing; best-effort cancellation is enough here.
      }
    }

    window.runtime.Quit();
  };

  const launchCyberstab = () => {
    try {
      if (clientLaunchedRef.current) return;
      clientLaunchedRef.current = true;
      window.go.main.App.LaunchClient(installDir);
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

    // Stage 1: verify credentials against selected DB engine.
    if (stage === 1 && needsPG && dbInstalled) {
      setPgVerifying(true);
      try {
        if (dbEngineBinDir) {
          await window.go.main.App.SelectDbEngineBin(dbEngineBinDir);
        } else if (dbEngineKind) {
          await window.go.main.App.SelectDbEngine(dbEngineKind);
        }
        await window.go.main.App.VerifyPostgresPassword(pgUser, pgPass);
        setPgVerified(true);
        setPgVerifyError("");
        setPgVerifying(false);
        setStage((s) => (s < 5 ? ((s + 1) as any) : s));
      } catch (_e: any) {
        setPgVerified(false);
        setPgVerifying(false);
        const raw = String(_e?.message || _e || "").trim();
        setPgVerifyError(raw || "Не удалось проверить учётные данные СУБД.");
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
    const user = pgUser.trim();
    if (!user) {
      setResetError("Укажите пользователя PostgreSQL на шаге выше.");
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
        setPgVerified(true);
        setPgVerifyError("");
      } catch (e: any) {
        setPgVerified(false);
        setPgVerifyError(String(e?.message || e));
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

  // Determine if we're in installation progress (running but not done)
  const isInstalling = running && !installDone;
  const showInstallDone = installDone || previewDone;
  const progressLayout = running || showInstallDone;
  const compactStageLayout = !running && !showInstallDone && stage === 0;
  const isFinalInstallStep = stage === 4;
  const nextDisabled = uiLocked || !canNext || (isFinalInstallStep && !readyToStart);
  const previewSteps = buildInstallStepsPreview(true, "new", true).map((s) => ({ ...s, IsDone: true }));
  const visibleSteps = previewDone && steps.length === 0 ? previewSteps : steps;
  const hasStepErrors = visibleSteps.some((s) => s.Error);
  const showCopyProgress = visibleSteps.some((s) => s.Description === "Копирование файлов");
  const showDbProgress = visibleSteps.some((s) => s.Description === "Инициализация базы данных");

  return (
    <div className="app">
      {showResetModal && (
        <div className="modalOverlay">
          <div className="modal">
            <div className="cardTitle">Мастер установки</div>
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

      {showInstallDirModal && (
        <div className="modalOverlay">
          <div className="modal">
            <div className="cardTitle">Папка уже существует</div>
            <div className="hint">
              Папка <b>{installDir}</b> уже существует. Выберите режим установки:
            </div>
            <div className="hint" style={{ marginTop: 10 }}>
              <b>Оставить</b> — не копировать файлы с флешки, использовать уже установленные в этой папке.
              <br />
              <b>Переустановить</b> — удалить старые папки Cyberstab* и скопировать заново с носителя.
            </div>
            <div className="wizardFooter">
              <button
                className={cls("btnCancel", running && "disabled")}
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
            <img className="topLogo" src={cyberstabLogo} alt="" aria-hidden />
            <div className="topTitle">Установщик Киберстаб</div>
          </div>
          <div className="topWinButtons" style={{ ["--wails-draggable" as any]: "no-drag" }}>
            <button type="button" className="winBtn" onClick={() => window.runtime.WindowMinimise()} aria-label="Свернуть">—</button>
            {!showInstallDone && (
              <button type="button" className="winBtn winClose" onClick={() => onCancel()} aria-label="Закрыть">×</button>
            )}
          </div>
        </div>
      </div>

      <div className={cls("content", compactStageLayout && "contentCompact")}>
        <div className={cls("wizardPanel", compactStageLayout && "wizardPanelCompact", progressLayout && "wizardPanelProgress")}>
          {!progressLayout && (
            <div className="wizardHead">
              <div>
                <h1 className="wizardTitle">Мастер установки</h1>
                <p className="wizardStep">Шаг {Math.min(stage + 1, TOTAL_VISIBLE_STEPS)} из {TOTAL_VISIBLE_STEPS}</p>
              </div>
              <p className="wizardOs">ОС: {osLabel(info?.os)}</p>
            </div>
          )}

          {!running && !showInstallDone && (
            <>
              <h2 className="wizardSectionTitle">{STAGE_SECTIONS[stage]}</h2>
              <div className="wizardBody">
                {stage === 0 && (
                  <>
                    <div className="optionChipRow">
                      <OptionChip
                        label="Сервер"
                        checked={installServer}
                        disabled={uiLocked}
                        onChange={(v) => {
                          setInstallServer(v);
                          if (v) setInstallDB(false);
                        }}
                      />
                      <OptionChip
                        label="Клиент"
                        checked={installClients}
                        disabled={uiLocked}
                        onChange={setInstallClients}
                      />
                      <OptionChip
                        label="Базу данных"
                        checked={installDB}
                        disabled={uiLocked}
                        onChange={(v) => {
                          setInstallDB(v);
                          if (v) setInstallServer(false);
                        }}
                      />
                      {usbChecking && (
                        <span className="usbStatusInline">Поиск USB-накопителя с дистрибутивом…</span>
                      )}
                    </div>
                    <p className="wizardHint" style={{ marginTop: 14 }}>
                      {installDB
                        ? "Будут скопированы только serverconsole и dbupdater для инициализации/восстановления базы данных."
                        : info?.os === "windows"
                          ? "Клиент будет выбран автоматически по разрядности системы (Windows64 на 64-bit)."
                          : "Клиент будет выбран автоматически (Linux64 на 64-bit)."}
                    </p>
                    {usbError && <p className="wizardHint wizardHintError">{usbError}</p>}
                  </>
                )}

                {stage === 1 && (
                  <>
                    {!needsPG ? (
                      <p className="wizardHint">Сервер/БД не выбран — шаг СУБД будет пропущен.</p>
                    ) : !dbInstalled ? (
                      <>
                        <p className="wizardHint wizardHintError">
                          Не найдены PostgreSQL или Jatoba в стандартных путях.
                        </p>
                        <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 10 }}>
                          {info?.os === "linux" ? (
                            pgPackageOptions.length > 0 ? (
                              <>
                                <p className="wizardHint">Доступные версии PostgreSQL в репозитории:</p>
                                {pgPackageOptions.map((pkg) => (
                                  <button
                                    key={pkg.version}
                                    type="button"
                                    className={cls("btnPrimary", pgInstalling && "disabled")}
                                    onClick={() => onInstallPgPackage(pkg.version)}
                                    disabled={pgInstalling}
                                  >
                                    {pgInstalling ? "Установка PostgreSQL…" : `Установить ${pkg.label}`}
                                  </button>
                                ))}
                              </>
                            ) : (
                              !pgInstallerFound && (
                                <p className="wizardHint">Пакеты PostgreSQL в репозитории не найдены.</p>
                              )
                            )
                          ) : (
                            pgInstallerFound && (
                              <button
                                type="button"
                                className={cls("btnPrimary", pgInstalling && "disabled")}
                                onClick={onInstallPgFromUsb}
                                disabled={pgInstalling}
                              >
                                {pgInstalling ? "Установка PostgreSQL…" : "Установить PostgreSQL с флешки"}
                              </button>
                            )
                          )}
                          {info?.os !== "linux" && !pgInstallerFound && (
                            <p className="wizardHint">Установщик PostgreSQL (postgresql*.exe) не найден на USB-накопителе.</p>
                          )}
                          <button type="button" className="btn" onClick={onPickDbDir} disabled={pgInstalling}>
                            Указать путь до СУБД вручную…
                          </button>
                        </div>
                        {pgInstallError && <p className="wizardHint wizardHintError">{pgInstallError}</p>}
                      </>
                    ) : (
                      <>
                        {dbEngines.length > 1 ? (
                          <div className="radioList" style={{ marginTop: 4, marginBottom: 10 }}>
                            {dbEngines.map((e) => (
                              <RadioRow
                                key={`${e.kind}:${e.binDir}`}
                                name="dbEngine"
                                label={e.label || dbEngineLabel(e.kind)}
                                checked={dbEngineBinDir === e.binDir}
                                onChange={async () => {
                                  setDbEngineKind(e.kind);
                                  setDbEngineBinDir(e.binDir);
                                  setPgVerified(false);
                                  setPgVerifyError("");
                                  await window.go.main.App.SelectDbEngineBin(e.binDir);
                                }}
                                disabled={uiLocked}
                              />
                            ))}
                          </div>
                        ) : (
                          <p className="wizardHint">Найдена СУБД: <b>{dbEngineLabel(dbEngineKind || dbEngines[0]?.kind, dbEngines)}</b>.</p>
                        )}
                        <p className="wizardHint">
                          Укажите пользователя СУБД с правами суперпользователя — он нужен для создания ролей и инициализации базы данных.
                        </p>
                        <div className="pgRow">
                          <div className="pgPasswordGroup">
                            <div className={cls("passwordField", pgVerified && "inputOk", pgVerifyError && "inputError")}>
                              <input
                                className="input passwordInput"
                                type="text"
                                value={pgUser}
                                onChange={(e) => {
                                  setPgUser(e.target.value);
                                  setPgVerified(false);
                                  setPgVerifyError("");
                                }}
                                onKeyDown={(e) => {
                                  if (e.key === "Enter") onNext();
                                }}
                                placeholder="Пользователь PostgreSQL"
                                disabled={uiLocked}
                                autoComplete="username"
                              />
                            </div>
                            <div className={cls("passwordField", pgVerified && "inputOk", pgVerifyError && "inputError")}>
                              <input
                                className="input passwordInput"
                                type={showPgPass ? "text" : "password"}
                                value={pgPass}
                                onChange={(e) => {
                                  setPgPass(e.target.value);
                                  setPgVerified(false);
                                  setPgVerifyError("");
                                }}
                                onKeyDown={(e) => {
                                  if (e.key === "Enter") onNext();
                                }}
                                placeholder="Пароль"
                                disabled={uiLocked}
                                autoComplete="current-password"
                              />
                              <button
                                type="button"
                                className="passwordEye"
                                onClick={() => setShowPgPass((v) => !v)}
                                disabled={uiLocked}
                                aria-label={showPgPass ? "Скрыть пароль" : "Показать пароль"}
                              >
                                <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                                  <path d="M2.8 12s3.4-6 9.2-6 9.2 6 9.2 6-3.4 6-9.2 6-9.2-6-9.2-6Z" />
                                  <circle cx="12" cy="12" r="2.6" />
                                </svg>
                              </button>
                            </div>
                            {pgVerifyError && <p className="pgInlineError" title={pgVerifyError}>{pgVerifyError}</p>}
                          </div>
                          {info?.postgresInstalled && (
                            <button
                              type="button"
                              className={cls("btnLink", uiLocked && "disabled")}
                              onClick={() => {
                                setShowResetModal(true);
                                setResetError("");
                                setNewPgPass("");
                                setNewPgPass2("");
                              }}
                              disabled={uiLocked || pgUser.trim().length === 0}
                              title={pgUser.trim().length === 0 ? "Сначала укажите пользователя PostgreSQL" : undefined}
                            >
                              Забыли пароль?
                            </button>
                          )}
                          {dbEngines.length === 1 && (
                            <button
                              type="button"
                              className={cls("btnLink", uiLocked && "disabled")}
                              onClick={onPickDbDir}
                              disabled={uiLocked}
                            >
                              Добавить вторую СУБД по пути…
                            </button>
                          )}
                        </div>
                        {pgVerified && <p className="wizardHint wizardHintSuccess">Подключение и права подтверждены</p>}
                      </>
                    )}
                  </>
                )}

                {stage === 2 && (
                  <>
                    {info?.os === "windows" ? (
                      <>
                        <p className="wizardHint">На Windows можно выбрать папку установки.</p>
                        <div className="pathRow">
                          <input
                            className="input"
                            value={installDir}
                            onChange={(e) => setInstallDir(e.target.value)}
                            placeholder="C:\Program Files\Cyberstab"
                            disabled={uiLocked}
                          />
                          <button type="button" className="btnSecondary" onClick={onPickInstallDir} disabled={uiLocked}>
                            Выбрать…
                          </button>
                        </div>
                      </>
                    ) : (
                      <p className="wizardHint">На Linux папка фиксированная: /opt/cyberstab</p>
                    )}
                  </>
                )}

                {stage === 3 && (
                  <>
                    <p className="wizardHint">
                      Выберите родительскую папку, где лежат папки CyberstabServer* / CyberstabClient*.
                    </p>
                    <div className="pathRow">
                      <input
                        className="input"
                        value={sourceRoot}
                        onChange={(e) => setSourceRoot(e.target.value)}
                        placeholder="Папка-дистрибутив"
                        disabled={uiLocked}
                      />
                      <button type="button" className="btnSecondary" onClick={onPickFolder} disabled={uiLocked}>
                        Выбрать…
                      </button>
                    </div>
                  </>
                )}

                {stage === 4 && (
                  <>
                    {!needsPG ? (
                      <p className="wizardHint">Сервер/БД не выбран — настройка БД пропущена.</p>
                    ) : dbChecking ? (
                      <p className="wizardHint">Проверка базы данных okidoci_db…</p>
                    ) : !dbExists ? (
                      <>
                        <p className="wizardHint">База данных okidoci_db не найдена. Выберите действие:</p>
                        <div className="radioList">
                          <RadioRow name="dbAction" label="Создать новую БД" checked={dbAction === "new"} onChange={() => setDbAction("new")} disabled={uiLocked} />
                          <RadioRow name="dbAction" label="Восстановить из .sql файла" checked={dbAction === "restore"} onChange={() => setDbAction("restore")} disabled={uiLocked} />
                        </div>
                        {dbAction === "restore" && (
                          <div className="restoreSqlRow">
                            <input
                              className="input restoreSqlInput"
                              value={restoreSqlPath}
                              onChange={(e) => setRestoreSqlPath(e.target.value)}
                              placeholder="Путь к .sql файлу"
                              disabled={uiLocked}
                            />
                            <button type="button" className="btnPrimary restoreSqlButton" onClick={onPickSqlFile} disabled={uiLocked}>
                              Выбрать…
                            </button>
                          </div>
                        )}
                      </>
                    ) : (
                      <>
                        <p className="wizardHint">База данных okidoci_db уже существует. Выберите действие:</p>
                        <div className="radioList">
                          <RadioRow
                            name="dbAction"
                            label="Оставить текущую БД (пропустить инициализацию)"
                            checked={dbAction === "skip"}
                            onChange={() => setDbAction("skip")}
                            disabled={uiLocked}
                          />
                          <RadioRow
                            name="dbAction"
                            label="Удалить + создать новую (резервная копия будет сохранена)"
                            checked={dbAction === "new"}
                            onChange={() => setDbAction("new")}
                            disabled={uiLocked}
                          />
                          <RadioRow
                            name="dbAction"
                            label="Удалить + восстановить из .sql файла (резервная копия будет сохранена)"
                            checked={dbAction === "restore"}
                            onChange={() => setDbAction("restore")}
                            disabled={uiLocked}
                          />
                        </div>
                        {dbAction === "restore" && (
                          <div className="restoreSqlRow">
                            <input
                              className="input restoreSqlInput"
                              value={restoreSqlPath}
                              onChange={(e) => setRestoreSqlPath(e.target.value)}
                              placeholder="Путь к .sql файлу"
                              disabled={uiLocked}
                            />
                            <button type="button" className="btnPrimary restoreSqlButton" onClick={onPickSqlFile} disabled={uiLocked}>
                              Выбрать…
                            </button>
                          </div>
                        )}
                      </>
                    )}
                  </>
                )}

                {stage === 5 && (
                  <>
                    <p className="wizardHint">Проверьте параметры и нажмите «Начать установку».</p>
                    <div className="wizardHeadActions">
                      <button
                        type="button"
                        className={cls("btnPrimary", (!readyToStart || uiLocked) && "disabled")}
                        onClick={onStart}
                        disabled={!readyToStart || uiLocked}
                      >
                        {running ? "Устанавливаю…" : "Начать установку"}
                      </button>
                    </div>
                    {info?.needAdmin && (
                      <p className="wizardHint wizardHintError">Запустите установщик от администратора.</p>
                    )}
                  </>
                )}

                {stage !== 1 && errorText && !running && (
                  <p className="wizardHint wizardHintError" style={{ marginTop: 12 }}>{errorText}</p>
                )}
              </div>
            </>
          )}

          {(running || showInstallDone) && (
            <>
              <h2 className="wizardSectionTitle">Ход установки</h2>
              <div className="wizardBody">
                <div className={cls("progressContent", showInstallDone && "progressContentDone")}>
                  <div className="progressLeft">
                    <div className="steps">
                      {visibleSteps.map((s) => (
                        <div key={`${s.Index}-${s.Description}`} className={cls("step", s.IsActive && "active", s.IsDone && "done", !!s.Error && "error")}>
                          <div className="stepIcon">{s.Error ? "✗" : s.IsDone ? "✓" : ""}</div>
                          <div className="stepBody">
                            <div className="stepTitle">{s.Description}</div>
                            {s.Status && <div className="stepStatus">{s.Status}</div>}
                            {s.Error && <div className="stepStatus" style={{ color: "var(--error)" }}>{s.Error}</div>}
                          </div>
                        </div>
                      ))}
                    </div>

                    {showInstallDone && installClients && (
                      <label className="check doneOpenCheck">
                        <input type="checkbox" checked={openCyberstab} onChange={(e) => setOpenCyberstab(e.target.checked)} />
                        <span>Открыть Киберстаб</span>
                      </label>
                    )}

                    {showCopyProgress && deployStatus && deployPct < 100 && (
                      <div className="initProgressSection">
                        <div className="initProgressLabel">
                          <span>Копирование файлов</span>
                          <span>{Math.round(deployPct)}%</span>
                        </div>
                        <div className="initProgressBar">
                          <div className={cls("initProgressBarFill", deployPct >= 99 && "done")} style={{ width: `${deployPct}%` }} />
                        </div>
                        <div className="initProgressStatus">{deployStatus}</div>
                      </div>
                    )}

                    {showDbProgress && initStatus && initPct < 100 && (
                      <div className="initProgressSection">
                        <div className="initProgressLabel">
                          <span>Инициализация базы данных</span>
                          <span>{Math.round(initPct)}%</span>
                        </div>
                        <div className="initProgressBar">
                          <div className={cls("initProgressBarFill", initPct >= 99 && "done")} style={{ width: `${initPct}%` }} />
                        </div>
                        <div className="initProgressStatus">{initStatus}</div>
                      </div>
                    )}
                  </div>

                  {showInstallDone && (
                  <div className="doneHero">
                    <div className="doneHeroIcon" style={{ color: hasStepErrors ? "var(--error)" : "var(--success)" }}>
                      {hasStepErrors ? "!" : "✓"}
                    </div>
                    <div className="doneHeroTitle">
                      {hasStepErrors ? "Установка завершена с ошибками" : "Установка завершена"}
                    </div>
                    {hasStepErrors && (
                      <p className="wizardHint wizardHintError">Проверьте детали в списке шагов выше.</p>
                    )}
                  </div>
                  )}
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {!showInstallDone && !isInstalling && (
        <div className="footerBar">
          <div className="footerInner">
            <button type="button" className="btnCancel" onClick={onCancel}>
              Отмена
            </button>
            <div className="footerRight">
              <button
                type="button"
                className={cls("btnFooterBack", (uiLocked || stage === 0) && "disabled")}
                onClick={() => {
                  if (uiLocked || stage === 0) return;
                  setStage((s) => (s > 0 ? ((s - 1) as any) : s));
                }}
                disabled={uiLocked || stage === 0}
                aria-label="Назад"
              >
                <svg viewBox="0 0 8 12" aria-hidden="true">
                  <path d="M6.5 1.5 2 6l4.5 4.5" />
                </svg>
              </button>
              <button
                type="button"
                className={cls("btnPrimary", nextDisabled && "disabled")}
                onClick={isFinalInstallStep ? onStart : onNext}
                disabled={nextDisabled}
              >
                {usbChecking ? "Поиск флешки…" : pgVerifying ? "Проверяю…" : isFinalInstallStep ? "Установить" : "Далее"}
              </button>
            </div>
          </div>
        </div>
      )}

      {isInstalling && (
        <div className="footerBar">
          <div className="footerInner">
            <button type="button" className="btnCancel" onClick={onCancel}>
              Отмена
            </button>
          </div>
        </div>
      )}

      {showInstallDone && (
        <div className="footerBar">
          <div className="footerInner doneFooter">
            <button type="button" className="btnPrimary" onClick={onCancel} style={{ minWidth: 160 }}>
              {openCyberstab ? "Закрыть и открыть" : "Закрыть"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
