import React, { useEffect, useMemo, useRef, useState } from "react";

function cls(...xs: (string | false | undefined | null)[]) {
  return xs.filter(Boolean).join(" ");
}

const STATUS_POLL_MS = 3000;
const INSTALL_DIR = "C:\\Program Files\\Cyberstab";

const IconPlay = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden>
    <path d="M8 5.14v13.72c0 .88 1.02 1.4 1.77.87l10.2-6.86a1 1 0 0 0 0-1.74l-10.2-6.86A1 1 0 0 0 8 5.14Z" />
  </svg>
);

const IconStop = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden>
    <rect x="7" y="7" width="10" height="10" rx="1.5" />
  </svg>
);

const IconRestart = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
    <path d="M3 12a9 9 0 0 1 15.5-6.36" />
    <path d="M18 3v5h-5" />
    <path d="M21 12a9 9 0 0 1-15.5 6.36" />
    <path d="M6 21v-5H1" />
  </svg>
);

type ManagerTab = "overview" | "users";

type ServerStatus = {
  taskExists: boolean;
  running: boolean;
  raw?: string;
  networkPort?: number;
  managementPort?: number;
  propertiesPath?: string;
};

type ServerSession = {
  userId: number;
  login: string;
  username: string;
  ip: string;
  company: string;
  module: string;
};

type ServerInfo = {
  status: ServerStatus;
  sessions?: ServerSession[];
  version?: string;
  sessionCount?: number;
};

type UpdateStep = "auth" | "result" | "done";

export const ServerManagerApp: React.FC = () => {
  const [info, setInfo] = useState<ServerInfo | null>(null);
  const [tab, setTab] = useState<ManagerTab>("overview");
  const [busy, setBusy] = useState(false);
  const [errorText, setErrorText] = useState("");
  const [internetOnline, setInternetOnline] = useState<boolean | null>(null);
  const [showUpdateModal, setShowUpdateModal] = useState(false);
  const [updateBusy, setUpdateBusy] = useState(false);
  const [updateError, setUpdateError] = useState("");
  const [updateStep, setUpdateStep] = useState<UpdateStep>("auth");
  const [ncLogin, setNcLogin] = useState("");
  const [ncPassword, setNcPassword] = useState("");
  const [rememberNc, setRememberNc] = useState(false);
  const [usedSavedCreds, setUsedSavedCreds] = useState(false);
  const [updateCheck, setUpdateCheck] = useState<{
    currentVersion: string;
    remoteVersion: string;
    updateRequired: boolean;
    archiveName: string;
    message: string;
  } | null>(null);
  const [checkLoading, setCheckLoading] = useState(false);
  const [updateDoneMessage, setUpdateDoneMessage] = useState("");
  const [showBackupModal, setShowBackupModal] = useState(false);
  const [backupModalPath, setBackupModalPath] = useState("");
  const pollInFlight = useRef(false);

  const pill = useMemo(() => {
    const st = info?.status;
    if (!st) return { text: "Неизвестно", kind: "unknown" as const };
    return st.running ? { text: "Активен", kind: "up" as const } : { text: "Остановлен", kind: "down" as const };
  }, [info]);

  const sessions = info?.sessions ?? [];
  const sessionCount = info?.sessionCount ?? sessions.length;
  const isRunning = !!info?.status?.running;
  const isStatusUnknown = !info?.status;

  const refresh = async (opts?: { silent?: boolean }) => {
    if (pollInFlight.current) return;
    pollInFlight.current = true;
    const silent = opts?.silent !== false;
    if (!silent) setErrorText("");
    try {
      const st = await window.go.main.App.GetServerInfo(INSTALL_DIR, "");
      setInfo(st);
    } catch (e: any) {
      if (!silent) setErrorText(String(e?.message || e));
    } finally {
      pollInFlight.current = false;
    }
  };

  useEffect(() => {
    void refresh({ silent: true });
    const t = window.setInterval(() => void refresh({ silent: true }), STATUS_POLL_MS);
    return () => window.clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const check = async () => {
      try {
        const online = await window.go.main.App.CheckInternet();
        setInternetOnline(online);
      } catch {
        setInternetOnline(false);
      }
    };
    void check();
    const t = window.setInterval(() => void check(), 60000);
    return () => window.clearInterval(t);
  }, []);

  const onStart = async () => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.StartServer(INSTALL_DIR);
      await refresh({ silent: false });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const onStop = async () => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.StopServer(INSTALL_DIR);
      await refresh({ silent: false });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const onRestart = async () => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.RestartServer(INSTALL_DIR);
      await refresh({ silent: false });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const onDisconnectUser = async (userId: number) => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.DisconnectServerUser(INSTALL_DIR, userId);
      await refresh({ silent: false });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const onDisconnectAll = async () => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.DisconnectAllServerUsers(INSTALL_DIR);
      await refresh({ silent: false });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const updateOnline = internetOnline !== false;
  const usersConnected = isRunning && sessionCount > 0;

  const updateResultText = useMemo(() => {
    if (!updateCheck) return "";
    if (!updateCheck.updateRequired) {
      const version = updateCheck.currentVersion || updateCheck.remoteVersion;
      return `У вас актуальная версия ${version}.`;
    }
    return `Доступно обновление ${updateCheck.remoteVersion}.`;
  }, [updateCheck]);

  const updateBlockedReason = useMemo(() => {
    if (!updateCheck?.updateRequired || updateStep !== "result") return "";
    if (usersConnected) {
      return `Отключите всех пользователей (${sessionCount}) перед обновлением.`;
    }
    return "";
  }, [updateCheck, updateStep, usersConnected, sessionCount]);

  const backupPathParts = useMemo(() => {
    const path = backupModalPath.trim();
    if (!path) return { fileName: "", dir: "" };
    const sep = path.includes("\\") ? "\\" : "/";
    const i = path.lastIndexOf(sep);
    if (i < 0) return { fileName: path, dir: "" };
    return { fileName: path.slice(i + 1), dir: path.slice(0, i) };
  }, [backupModalPath]);

  const canStartUpdate = !!updateCheck?.updateRequired && !usersConnected;

  const onBackupDatabase = async () => {
    setBusy(true);
    setErrorText("");
    setShowBackupModal(false);
    try {
      const result = await window.go.main.App.BackupServerDatabase(INSTALL_DIR);
      if (result.path) {
        try {
          await window.go.main.App.RevealPathInExplorer(result.path);
        } catch {
          // проводник не критичен
        }
      }
      setBackupModalPath(result.path || "");
      setShowBackupModal(true);
      await refresh({ silent: true });
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const openUpdateModal = async () => {
    setUpdateError("");
    setUpdateDoneMessage("");
    setOfflinePath("");
    setUpdateCheck(null);
    setUpdateStep("auth");
    setUsedSavedCreds(false);
    setNcPassword("");
    setRememberNc(false);
    setShowUpdateModal(true);

    if (!updateOnline) return;

    try {
      const savedLogin = await window.go.main.App.GetNextcloudSavedLogin();
      if (!savedLogin) return;

      setNcLogin(savedLogin);
      setUpdateStep("result");
      setCheckLoading(true);
      try {
        const check = await window.go.main.App.CheckNextcloudServerUpdateWithSaved(INSTALL_DIR);
        setUpdateCheck(check);
        setUsedSavedCreds(true);
      } catch {
        setUpdateStep("auth");
        setRememberNc(true);
        setUpdateCheck(null);
      } finally {
        setCheckLoading(false);
      }
    } catch (e: any) {
      setUpdateError(String(e?.message || e));
    }
  };

  const closeUpdateModal = () => {
    if (updateBusy || checkLoading) return;
    setShowUpdateModal(false);
    setUpdateError("");
    setUpdateDoneMessage("");
    setUpdateStep("auth");
    setNcPassword("");
    setUpdateCheck(null);
    setUsedSavedCreds(false);
  };

  const onPickUpdateArchive = async () => {
    try {
      const p = await window.go.main.App.PickUpdateArchive();
      if (p) setOfflinePath(p);
    } catch (e: any) {
      setUpdateError(String(e?.message || e));
    }
  };

  const onPickUpdateFolder = async () => {
    try {
      const p = await window.go.main.App.PickUpdateFolder();
      if (p) setOfflinePath(p);
    } catch (e: any) {
      setUpdateError(String(e?.message || e));
    }
  };

  const onCheckUpdate = async () => {
    if (!ncLogin.trim() || !ncPassword) {
      setUpdateError("Укажите логин и пароль");
      return;
    }
    setCheckLoading(true);
    setUpdateError("");
    setUpdateCheck(null);
    try {
      const check = await window.go.main.App.CheckNextcloudServerUpdate(
        INSTALL_DIR,
        ncLogin.trim(),
        ncPassword
      );
      await window.go.main.App.SaveNextcloudCredentials(ncLogin.trim(), ncPassword, rememberNc);
      setUpdateCheck(check);
      setUsedSavedCreds(false);
      setUpdateStep("result");
    } catch (e: any) {
      setUpdateError(String(e?.message || e));
    } finally {
      setCheckLoading(false);
    }
  };

  const onRunUpdate = async () => {
    setUpdateBusy(true);
    setUpdateError("");
    try {
      if (usersConnected) {
        throw new Error(`Отключите всех пользователей (${sessionCount}) перед обновлением`);
      }
      const online = internetOnline !== false;
      let result;
      if (online) {
        if (updateCheck && !updateCheck.updateRequired) {
          throw new Error("Обновление не требуется");
        }
        if (usedSavedCreds && !ncPassword) {
          result = await window.go.main.App.RunServerUpdateFromNextcloudSaved(INSTALL_DIR, false);
        } else {
          if (!ncLogin.trim() || !ncPassword) {
            throw new Error("Укажите логин и пароль");
          }
          result = await window.go.main.App.RunServerUpdateFromNextcloud(
            INSTALL_DIR,
            ncLogin.trim(),
            ncPassword,
            rememberNc,
            false
          );
        }
      } else {
        if (!offlinePath.trim()) {
          throw new Error("Укажите путь к архиву");
        }
        result = await window.go.main.App.RunServerUpdateFromPath(INSTALL_DIR, offlinePath.trim(), false);
      }
      setUpdateDoneMessage(result.message || "Сервер успешно обновлён");
      setUpdateStep("done");
      setNcPassword("");
      await refresh({ silent: true });
    } catch (e: any) {
      setUpdateError(String(e?.message || e));
    } finally {
      setUpdateBusy(false);
    }
  };

  return (
    <div className="app managerApp">
      <div className="topBar">
        <div className="topBarInner" style={{ ["--wails-draggable" as any]: "drag" }}>
          <div className="topLeft">
            <div className="topLogo">C</div>
            <div className="topTitle">Управление сервером</div>
          </div>
          <div className="topWinButtons" style={{ ["--wails-draggable" as any]: "no-drag" }}>
            <button className="winBtn" onClick={() => window.runtime.WindowMinimise()}>—</button>
            <button className="winBtn close" onClick={() => window.runtime.Quit()}>×</button>
          </div>
        </div>
      </div>

      {internetOnline === false && (
        <div className="managerInternetBanner">
          Подключение к интернету не обнаружено. Управление локальным сервером доступно.
        </div>
      )}

      <div className="managerTabs">
        <button
          type="button"
          className={cls("managerTab", tab === "overview" && "active")}
          onClick={() => setTab("overview")}
        >
          Обзор
        </button>
        <button
          type="button"
          className={cls("managerTab", tab === "users" && "active")}
          onClick={() => setTab("users")}
        >
          Пользователи{sessionCount > 0 ? ` (${sessionCount})` : ""}
        </button>
      </div>

      <div className="content managerContent">
        <div className="managerScroll">
          {tab === "overview" ? (
            <div className="grid managerGrid">
              <div className="card">
                <div className="cardTitle">Статус</div>
                <div className="serverRow">
                  <div className={cls("statusDot", pill.kind)} />
                  <div className="serverStatusText">{pill.text}</div>
                  <div className="managerIconActions">
                    {!isRunning ? (
                      <button
                        type="button"
                        className={cls("managerIconBtn", "managerIconBtnPrimary", (busy || isStatusUnknown) && "disabled")}
                        onClick={onStart}
                        disabled={busy || isStatusUnknown}
                        title={isStatusUnknown ? "Определение статуса сервера…" : "Запустить сервер"}
                        aria-label={isStatusUnknown ? "Определение статуса сервера" : "Запустить сервер"}
                      >
                        <IconPlay />
                      </button>
                    ) : (
                      <>
                        <button
                          type="button"
                          className={cls("managerIconBtn", busy && "disabled")}
                          onClick={onRestart}
                          disabled={busy}
                          title="Перезапустить сервер"
                          aria-label="Перезапустить сервер"
                        >
                          <IconRestart />
                        </button>
                        <button
                          type="button"
                          className={cls("managerIconBtn", "managerIconBtnDanger", busy && "disabled")}
                          onClick={onStop}
                          disabled={busy}
                          title="Остановить сервер"
                          aria-label="Остановить сервер"
                        >
                          <IconStop />
                        </button>
                      </>
                    )}
                  </div>
                </div>
                {errorText && tab === "overview" && (
                  <div className="hint" style={{ marginTop: 10, color: "#b42318" }}>
                    {errorText}
                  </div>
                )}

                <div className="serverMeta">
                  <div className="metaLine">
                    <span className="metaLabel">Порт сервера</span>
                    <span className="metaValue metaValuePlain">
                      {info?.status?.networkPort ?? "—"}
                    </span>
                  </div>
                  <div className="metaLine">
                    <span className="metaLabel">Версия сервера</span>
                    <span className="metaValue metaValuePlain">{info?.version || "—"}</span>
                  </div>
                  <div className="metaLine">
                    <span className="metaLabel">Пользователи в Киберстаб</span>
                    <span className="metaValue">{isRunning ? sessionCount : "—"}</span>
                  </div>
                </div>
              </div>

              <div className="card">
                <div className="cardTitle">Управление</div>
                <div className="managerActions managerActionsCompact managerActionsRow">
                  <button
                    type="button"
                    className={cls("btnPrimary", "managerUpdateBtn", busy && "disabled")}
                    disabled={busy}
                    onClick={() => void openUpdateModal()}
                  >
                    Обновить сервер
                  </button>
                  <button
                    type="button"
                    className={cls("btn", "managerBackupBtn", busy && "disabled")}
                    disabled={busy}
                    onClick={() => void onBackupDatabase()}
                  >
                    Резервная копия БД
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="grid managerGrid">
              <div className="card">
                <div className="usersCardHead">
                  <div>
                    <div className="cardTitle">Пользователи</div>
                    <div className="hint" style={{ marginTop: 6 }}>
                      Активные сессии клиентов Киберстаб.
                    </div>
                  </div>
                  <button
                    type="button"
                    className={cls("btnDanger", (busy || !isRunning || sessionCount === 0) && "disabled")}
                    onClick={onDisconnectAll}
                    disabled={busy || !isRunning || sessionCount === 0}
                  >
                    Отключить всех
                  </button>
                </div>

                {!isRunning ? (
                  <div className="usersEmpty">Сервер остановлен — список пользователей недоступен.</div>
                ) : sessions.length === 0 ? (
                  <div className="usersEmpty">Нет активных подключений.</div>
                ) : (
                  <div className="usersTableWrap">
                    <table className="usersTable">
                      <thead>
                        <tr>
                          <th>Логин</th>
                          <th>Имя</th>
                          <th>IP</th>
                          <th>Модуль</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {sessions.map((s) => (
                          <tr key={s.userId}>
                            <td>{s.login}</td>
                            <td>{s.username}</td>
                            <td>{s.ip}</td>
                            <td>{s.module}</td>
                            <td className="usersTableActions">
                              <button
                                type="button"
                                className={cls("btn", busy && "disabled")}
                                onClick={() => void onDisconnectUser(s.userId)}
                                disabled={busy}
                              >
                                Отключить
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {errorText && (
                  <div className="hint" style={{ marginTop: 10, color: "#b42318" }}>
                    {errorText}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

      {showBackupModal && (
        <div className="modalOverlay">
          <div className="modal managerBackupModal">
            <div className="managerBackupSuccessHead">
              <div className="managerBackupSuccessIcon" aria-hidden>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20 6 9 17l-5-5" />
                </svg>
              </div>
              <div>
                <div className="managerBackupTitle">Резервная копия создана</div>
                <div className="managerBackupSubtitle">Файл базы данных сохранён на диск</div>
              </div>
            </div>

            {backupModalPath && (
              <div className="managerBackupPathBox">
                <div className="managerBackupPathLabel">Файл</div>
                <div className="managerBackupPathFile">{backupPathParts.fileName}</div>
                {backupPathParts.dir && (
                  <div className="managerBackupPathDir">{backupPathParts.dir}</div>
                )}
              </div>
            )}

            <div className="wizardFooter">
              <button
                type="button"
                className="btnSecondary"
                onClick={() => {
                  if (backupModalPath) {
                    void window.go.main.App.RevealPathInExplorer(backupModalPath);
                  }
                }}
              >
                Открыть в проводнике
              </button>
              <button type="button" className="btnPrimary" onClick={() => setShowBackupModal(false)}>
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}

      {showUpdateModal && (
        <div className="modalOverlay">
          <div className="modal managerUpdateModal">
            <div className="cardTitle">Обновление сервера</div>

            {updateStep === "auth" && updateOnline && (
              <>
                <div className="row" style={{ marginTop: 4 }}>
                  <input
                    className="input"
                    value={ncLogin}
                    onChange={(e) => setNcLogin(e.target.value)}
                    placeholder="Логин"
                    disabled={updateBusy || checkLoading}
                    autoComplete="username"
                  />
                </div>
                <div className="row" style={{ marginTop: 10 }}>
                  <input
                    className="input"
                    type="password"
                    value={ncPassword}
                    onChange={(e) => setNcPassword(e.target.value)}
                    placeholder="Пароль"
                    disabled={updateBusy || checkLoading}
                    autoComplete="current-password"
                  />
                </div>
                <label className="checkRow" style={{ marginTop: 12 }}>
                  <input
                    type="checkbox"
                    checked={rememberNc}
                    onChange={(e) => setRememberNc(e.target.checked)}
                    disabled={updateBusy || checkLoading}
                  />
                  <span>Запомнить</span>
                </label>
              </>
            )}

            {updateStep === "auth" && !updateOnline && (
              <>
                <div className="row" style={{ marginTop: 4 }}>
                  <input
                    className="input"
                    value={offlinePath}
                    onChange={(e) => setOfflinePath(e.target.value)}
                    placeholder="Путь к архиву"
                    disabled={updateBusy}
                  />
                </div>
                <div className="managerUpdatePickRow" style={{ marginTop: 10 }}>
                  <button type="button" className={cls("btn", updateBusy && "disabled")} onClick={() => void onPickUpdateArchive()} disabled={updateBusy}>
                    Выбрать файл
                  </button>
                  <button type="button" className={cls("btn", updateBusy && "disabled")} onClick={() => void onPickUpdateFolder()} disabled={updateBusy}>
                    Выбрать папку
                  </button>
                </div>
              </>
            )}

            {updateStep === "result" && (
              <>
                <div className="hint managerUpdateResult">{checkLoading ? "Проверка…" : updateResultText}</div>
                {updateBlockedReason && (
                  <div className="hint" style={{ marginTop: 10, color: "#8a5b00" }}>
                    {updateBlockedReason}
                  </div>
                )}
              </>
            )}

            {updateStep === "done" && (
              <div className="hint managerUpdateResult" style={{ color: "var(--success)" }}>
                {updateDoneMessage}
              </div>
            )}

            {updateError && (
              <div className="hint" style={{ marginTop: 10, color: "#b42318" }}>
                {updateError}
              </div>
            )}

            {updateBusy && (
              <div className="hint" style={{ marginTop: 10, fontSize: 13, color: "var(--text-muted)" }}>
                Обновление сервера, запуск и обновление консоли…
              </div>
            )}

            <div className="wizardFooter">
              <button
                type="button"
                className={cls("btnCancel", (updateBusy || checkLoading) && "disabled")}
                onClick={closeUpdateModal}
                disabled={updateBusy || checkLoading}
              >
                {updateStep === "done" ? "Закрыть" : "Отмена"}
              </button>

              {updateStep === "auth" && updateOnline && (
                <button
                  type="button"
                  className={cls("btnPrimary", (updateBusy || checkLoading) && "disabled")}
                  onClick={() => void onCheckUpdate()}
                  disabled={updateBusy || checkLoading}
                >
                  {checkLoading ? "Проверка…" : "Проверить обновление"}
                </button>
              )}

              {updateStep === "auth" && !updateOnline && (
                <button
                  type="button"
                  className={cls("btnPrimary", updateBusy && "disabled")}
                  onClick={() => void onRunUpdate()}
                  disabled={updateBusy}
                >
                  {updateBusy ? "Обновление…" : "Обновить"}
                </button>
              )}

              {updateStep === "result" && updateCheck?.updateRequired && (
                <button
                  type="button"
                  className={cls("btnPrimary", (updateBusy || !canStartUpdate) && "disabled")}
                  onClick={() => void onRunUpdate()}
                  disabled={updateBusy || !canStartUpdate}
                >
                  {updateBusy ? "Обновление…" : "Обновить"}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
