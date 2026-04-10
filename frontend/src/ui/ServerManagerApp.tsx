import React, { useEffect, useMemo, useState } from "react";

function cls(...xs: (string | false | undefined | null)[]) {
  return xs.filter(Boolean).join(" ");
}

type ServerStatus = {
  taskExists: boolean;
  running: boolean;
  raw?: string;
};

type ServerInfo = {
  status: ServerStatus;
  connections?: string;
  version?: string;
  sessionCount?: number;
  consoleErr?: string;
  rawConsole?: string;
};

export const ServerManagerApp: React.FC = () => {
  const [installDir, setInstallDir] = useState("C:\\Program Files\\Cyberstab");
  const [pgPass, setPgPass] = useState("");
  const [info, setInfo] = useState<ServerInfo | null>(null);
  const [busy, setBusy] = useState(false);
  const [errorText, setErrorText] = useState("");
  const [showDetails, setShowDetails] = useState(false);

  const pill = useMemo(() => {
    const st = info?.status;
    if (!st) return { text: "Неизвестно", kind: "unknown" as const };
    if (!st.taskExists) return { text: "Задача автозапуска не найдена", kind: "down" as const };
    return st.running ? { text: "Активен", kind: "up" as const } : { text: "Остановлен", kind: "down" as const };
  }, [info]);

  const refresh = async () => {
    setErrorText("");
    try {
      const st = await window.go.main.App.GetServerInfo(installDir.trim(), pgPass);
      setInfo(st);
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    }
  };

  useEffect(() => {
    refresh();
    const t = window.setInterval(refresh, 30000);
    return () => window.clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onStart = async () => {
    setBusy(true);
    setErrorText("");
    try {
      await window.go.main.App.StartServer();
      await refresh();
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
      await window.go.main.App.StopServer(installDir.trim());
      await refresh();
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
      await window.go.main.App.RestartServer(installDir.trim());
      await refresh();
    } catch (e: any) {
      setErrorText(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const isRunning = !!info?.status?.taskExists && !!info?.status?.running;

  return (
    <div className="app">
      <div className="topBar">
        <div className="topBarInner">
          <div className="topLeft">
            <div className="topLogo">C</div>
            <div className="topTitle">Управление сервером</div>
          </div>
          <div className="topWinButtons">
            <button className="winBtn" onClick={() => window.runtime.WindowMinimise()}>—</button>
            <button className="winBtn close" onClick={() => window.runtime.Quit()}>×</button>
          </div>
        </div>
      </div>

      <div className="content">
        <div className="grid managerGrid">
          <div className="card">
            <div className="cardTitle">Статус</div>
            <div className="serverRow">
              <div className={cls("statusDot", pill.kind)} />
              <div className="serverStatusText">{pill.text}</div>
              <button type="button" className="btn" onClick={refresh} disabled={busy}>
                Обновить
              </button>
            </div>

            <div className="serverMeta">
              <div className="metaLine">
                <span className="metaLabel">Версия сервера</span>
                <span className="metaValue metaValuePlain">{info?.version || "—"}</span>
              </div>
              <div className="metaLine metaLineStack">
                <div className="metaLineHead">
                  <span className="metaLabel">Пользователи в Cyberstab</span>
                  <span className="metaValue">
                    {info?.sessionCount != null ? `${info.sessionCount}` : "—"}
                  </span>
                </div>
                <div className="sessionsHint">
                  Список обновляется автоматически и при нажатии «Обновить».
                </div>
                {info?.consoleErr && (
                  <div className="sessionsEmpty" style={{ color: "#b42318" }}>
                    Консоль: {info.consoleErr}
                  </div>
                )}
                {info?.connections ? (
                  <pre className="sessionsPre">{info.connections}</pre>
                ) : (
                  <div className="sessionsEmpty">
                    {!info?.consoleErr
                      ? "Нет данных — укажите пароль postgres (как при установке), нажмите «Обновить». Вывод берётся из консоли сервера и из console.log."
                      : null}
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="card">
            <div className="cardTitle">Управление</div>
            <div className="hint">
              Управление выполняется через задачу планировщика <b>{'cyberstab-server'}</b>.
            </div>
            <div className="managerActions">
              {!isRunning ? (
                <button
                  type="button"
                  className={cls("btnPrimary", busy && "disabled")}
                  onClick={onStart}
                  disabled={busy}
                >
                  Запустить сервер
                </button>
              ) : (
                <>
                  <button
                    type="button"
                    className={cls("btnDanger", busy && "disabled")}
                    onClick={onStop}
                    disabled={busy}
                  >
                    Остановить сервер
                  </button>
                  <button
                    type="button"
                    className={cls("btn", busy && "disabled")}
                    onClick={onRestart}
                    disabled={busy}
                  >
                    Перезапустить
                  </button>
                </>
              )}
            </div>
          </div>

          <div className="card">
            <div className="cardTitle">Настройки</div>
            <div className="row" style={{ marginTop: 8 }}>
              <input
                className="input"
                value={installDir}
                onChange={(e) => setInstallDir(e.target.value)}
                placeholder="Папка установки (для остановки процессов)"
                disabled={busy}
              />
            </div>
            <div className="row" style={{ marginTop: 10 }}>
              <input
                className="input"
                value={pgPass}
                onChange={(e) => setPgPass(e.target.value)}
                placeholder="Пароль postgres (для connections/sversion)"
                disabled={busy}
                type="password"
                onBlur={() => refresh()}
              />
            </div>
            <div className="row" style={{ marginTop: 10, justifyContent: "space-between" }}>
              <label className="check" style={{ padding: "8px 10px" }}>
                <input type="checkbox" checked={showDetails} onChange={(e) => setShowDetails(e.target.checked)} />
                <span>Показать детали</span>
              </label>
            </div>

            {errorText && (
              <div className="hint" style={{ marginTop: 10, color: "#b42318" }}>
                {errorText}
              </div>
            )}

            {showDetails && (
              <div className="detailsBox">
                <div className="detailsTitle">Детали</div>
                <div className="detailsMono">
                  {info?.status?.raw ? `schtasks: ${String(info.status.raw).slice(0, 800)}\n\n` : ""}
                  {info?.rawConsole ? `console: ${String(info.rawConsole).slice(0, 6000)}` : ""}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

