import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { installRuntimeShim } from "./api";
import "./styles.css";

installRuntimeShim();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
