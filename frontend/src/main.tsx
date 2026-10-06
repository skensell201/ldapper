import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import "./tokens.css";
import { apply, loadPref, resolve } from "./theme";

// The theme goes on before the first render, so the window never paints in
// the wrong one and then flips.
apply(resolve(loadPref()));

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
