import React, { useEffect, useState } from "react";
import ReactDOM from "react-dom/client";
import { App as InstallerApp } from "./ui/App";
import { UninstallApp } from "./ui/UninstallApp";
import { ServerManagerApp } from "./ui/ServerManagerApp";
import "./ui/styles.css";
import { applyFlexGapClass } from "./ui/flexGap";

applyFlexGapClass();

declare global {
  interface Window {
    go: any;
    runtime: any;
  }
}

function Root() {
  const [mode, setMode] = useState<"install" | "uninstall" | "manager" | "">("");

  useEffect(() => {
    (async () => {
      try {
        const m = await window.go.main.App.GetUIMode();
        if (m === "uninstall") setMode("uninstall");
        else if (m === "manager") setMode("manager");
        else setMode("install");
      } catch {
        setMode("install");
      }
    })();
  }, []);

  if (mode === "") {
    return (
      <div className="app">
        <div className="content" style={{ padding: 24 }}>
          <div className="hint">Загрузка…</div>
        </div>
      </div>
    );
  }

  if (mode === "uninstall") return <UninstallApp />;
  if (mode === "manager") return <ServerManagerApp />;
  return <InstallerApp />;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
);
