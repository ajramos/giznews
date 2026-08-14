import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { installRuntimeShim } from "./api";
import { initTheme } from "./theme";
import "./styles.css";

installRuntimeShim();
initTheme();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
