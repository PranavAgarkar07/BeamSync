// src/lib/wails.js
//
// Central Wails store for BeamSync.
//
// Every Wails call the frontend makes routes through this module instead of
// importing `../wailsjs/go/main/App.js` or `../wailsjs/runtime/runtime.js`
// directly, so a binding rename breaks at compile time rather than at runtime
// (PITFALLS 3a / T-05-01-01). Components import `./lib/wails.js`, never
// `window.go.main.App` or `window.runtime`.
//
// Generated bindings:
//   - ../wailsjs/go/main/App.js   (13 bound methods used by App.svelte)
//   - ../wailsjs/runtime/runtime.js (5 runtime event/helpers used by App.svelte)
//
// `runtime` re-exports `window.go.main.App` for the single direct call at
// App.svelte:273 (`appVersion = await window.go.main.App.GetVersion()`).

import {
  StartReceiverDefault,
  StartSender,
  SendFiles,
  PlaySound,
  OpenFile,
  ResetApp,
  GetReceivedFiles,
  GetSavePath,
  SetSavePath,
  GetTransferSettings,
  SaveTransferSettings,
  ApproveTransfer,
  RejectTransfer,
} from "../../wailsjs/go/main/App.js";

import {
  EventsOn,
  EventsOffAll,
  BrowserOpenURL,
  OnFileDrop,
  OnFileDropOff,
} from "../../wailsjs/runtime/runtime.js";

// `window.go.main.App` re-exported as `runtime` so App.svelte's direct call
// (`runtime.GetVersion()`) stays valid without touching the global directly.
export const runtime = window.go.main.App;

export {
  StartReceiverDefault,
  StartSender,
  SendFiles,
  PlaySound,
  OpenFile,
  ResetApp,
  GetReceivedFiles,
  GetSavePath,
  SetSavePath,
  GetTransferSettings,
  SaveTransferSettings,
  ApproveTransfer,
  RejectTransfer,
  EventsOn,
  EventsOffAll,
  BrowserOpenURL,
  OnFileDrop,
  OnFileDropOff,
};
