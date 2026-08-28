/**
 * BeamSync Neubrutalism Design System
 * ─────────────────────────────────────────────────────────────────
 * Barrel export — import everything from one place:
 *
 *   import { DeviceCard, FileDropZone, TransferProgressBar,
 *            ConnectedDevicesPanel, TopNavBar } from './design-system';
 *
 * CSS tokens must be imported separately in your root CSS/entry point:
 *   import './design-system/tokens.css';
 */

export { default as DeviceCard           } from './DeviceCard.svelte';
export { default as FileDropZone         } from './FileDropZone.svelte';
export { default as TransferProgressBar  } from './TransferProgressBar.svelte';
export { default as ConnectedDevicesPanel} from './ConnectedDevicesPanel.svelte';
export { default as TopNavBar            } from './TopNavBar.svelte';
export { default as TransferComplete     } from './TransferComplete.svelte';
export { default as ActivityPanel        } from './ActivityPanel.svelte';
export { default as TransferStatsDashboard } from './TransferStatsDashboard.svelte';
