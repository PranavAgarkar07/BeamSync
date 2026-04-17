<!--
  TransferComplete.svelte
  ─────────────────────────────────────────────────────────────────
  Razorpay / Google Pay–style transfer success animation.

  Animation sequence:
    0ms    Scrim fades in
    80ms   White card slides up + scales in (spring)
    300ms  Green circle scales in (spring overshoot)
    480ms  SVG ring draws itself (stroke-dashoffset)
    560ms  Checkmark path draws in
    700ms  "Transfer Complete" title fades + slides up
    860ms  File count badge pops in
   1000ms  Ripple pulse starts (2 rings expand + fade)
   3000ms  Auto-dismiss: card slides down + scrim fades

  Props:
    fileCount  {number}   How many files were received
    totalSize  {string}   e.g. "128 MB" (optional)
    show       {boolean}  Mount/unmount control

  Events:
    on:dismiss — fired when animation out completes
-->
<script>
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';

  export let fileCount  = 0;
  export let totalSize  = '';
  export let show       = false;

  const dispatch = createEventDispatcher();

  let mounted  = false;
  let entered  = false;  // card is visible
  let ringDraw = false;  // SVG ring + check
  let textIn   = false;  // title + badge
  let ripple   = false;  // expanding rings
  let leaving  = false;  // exit animation

  let t1, t2, t3, t4, t5, t6;

  function clearTimers() {
    [t1,t2,t3,t4,t5,t6].forEach(clearTimeout);
  }

  function runSequence() {
    clearTimers();
    entered  = false;
    ringDraw = false;
    textIn   = false;
    ripple   = false;
    leaving  = false;

    t1 = setTimeout(() => entered  = true,  80);
    t2 = setTimeout(() => ringDraw = true,  400);
    t3 = setTimeout(() => textIn   = true,  700);
    t4 = setTimeout(() => ripple   = true,  900);
    t5 = setTimeout(() => {
      leaving  = true;
      entered  = false;
    }, 3200);
    t6 = setTimeout(() => dispatch('dismiss'), 3700);
  }

  $: if (show && mounted) runSequence();

  onMount(() => { mounted = true; });
  onDestroy(clearTimers);
</script>

{#if show}
  <!-- Scrim -->
  <div
    class="tc-scrim"
    class:tc-scrim--in={entered || ringDraw || textIn}
    class:tc-scrim--out={leaving}
    role="status"
    aria-live="assertive"
    aria-label="Transfer complete. {fileCount} file{fileCount !== 1 ? 's' : ''} received{totalSize ? '. ' + totalSize + ' total' : ''}."
  >
    <!-- Card -->
    <div
      class="tc-card"
      class:tc-card--in={entered}
      class:tc-card--out={leaving}
    >

      <!-- Ripple rings (behind the circle) -->
      <div class="tc-ripple-wrap" aria-hidden="true">
        <div class="tc-ripple tc-ripple--1" class:tc-ripple--active={ripple}></div>
        <div class="tc-ripple tc-ripple--2" class:tc-ripple--active={ripple}></div>
      </div>

      <!-- Success circle + SVG checkmark -->
      <div
        class="tc-circle"
        class:tc-circle--in={entered}
        aria-hidden="true"
      >
        <svg class="tc-svg" viewBox="0 0 80 80" fill="none" xmlns="http://www.w3.org/2000/svg">
          <!-- Animated ring -->
          <circle
            class="tc-ring"
            class:tc-ring--draw={ringDraw}
            cx="40" cy="40" r="36"
            stroke="#00C875"
            stroke-width="3.5"
            stroke-linecap="round"
          />
          <!-- Checkmark -->
          <polyline
            class="tc-check"
            class:tc-check--draw={ringDraw}
            points="22,42 34,54 58,28"
            stroke="#00C875"
            stroke-width="5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </div>

      <!-- Text -->
      <div class="tc-text" class:tc-text--in={textIn}>
        <h2 class="tc-title">Transfer Complete</h2>
        <div class="tc-badge">
          <span class="tc-badge__count">{fileCount} file{fileCount !== 1 ? 's' : ''}</span>
          {#if totalSize}
            <span class="tc-badge__dot" aria-hidden="true">·</span>
            <span class="tc-badge__size">{totalSize}</span>
          {/if}
        </div>
      </div>

    </div>
  </div>
{/if}

<style>
  /* ── Scrim ──────────────────────────────────────────────────────── */
  .tc-scrim {
    position: fixed;
    inset: 0;
    z-index: var(--nb-z-overlay, 100);
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(8, 12, 24, 0);
    transition: background 300ms ease;
    pointer-events: none;
  }

  .tc-scrim--in {
    background: rgba(8, 12, 24, 0.65);
    pointer-events: all;
  }

  .tc-scrim--out {
    background: rgba(8, 12, 24, 0);
    pointer-events: none;
  }

  /* ── Card ───────────────────────────────────────────────────────── */
  .tc-card {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 28px;
    padding: 48px 56px 40px;
    background: var(--nb-surface, #ffffff);
    border: 3px solid var(--nb-border-color, #0A0A0A);
    box-shadow: 8px 8px 0px var(--nb-border-color, #0A0A0A);
    min-width: 300px;
    overflow: visible;

    /* Entry: slide up from below + scale */
    transform: translateY(48px) scale(0.94);
    opacity: 0;
    transition:
      transform 420ms cubic-bezier(0.34, 1.4, 0.64, 1),
      opacity   240ms ease-out;
  }

  .tc-card--in {
    transform: translateY(0) scale(1);
    opacity: 1;
  }

  .tc-card--out {
    transform: translateY(40px) scale(0.95);
    opacity: 0;
    transition:
      transform 320ms cubic-bezier(0.4, 0, 1, 1),
      opacity   280ms ease-in;
  }

  /* ── Ripple rings ───────────────────────────────────────────────── */
  .tc-ripple-wrap {
    position: absolute;
    top: 48px;   /* aligned with circle center */
    left: 50%;
    transform: translateX(-50%);
    width: 80px;
    height: 80px;
    pointer-events: none;
  }

  .tc-ripple {
    position: absolute;
    inset: -8px;
    border-radius: 50%;
    border: 2px solid #00C875;
    opacity: 0;
  }

  .tc-ripple--1.tc-ripple--active {
    animation: ripplePulse 1.6s cubic-bezier(0, 0, 0.2, 1) 0.1s infinite;
  }

  .tc-ripple--2.tc-ripple--active {
    animation: ripplePulse 1.6s cubic-bezier(0, 0, 0.2, 1) 0.55s infinite;
  }

  @keyframes ripplePulse {
    0%   { transform: scale(1);    opacity: 0.5; }
    100% { transform: scale(2.2);  opacity: 0; }
  }

  /* ── Circle ─────────────────────────────────────────────────────── */
  .tc-circle {
    width: 80px;
    height: 80px;
    background: #F0FFF8;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px solid #00C875;

    transform: scale(0.5);
    opacity: 0;
    transition:
      transform 380ms cubic-bezier(0.34, 1.5, 0.64, 1) 200ms,
      opacity   220ms ease-out 200ms;
  }

  .tc-circle--in {
    transform: scale(1);
    opacity: 1;
  }

  /* ── SVG ────────────────────────────────────────────────────────── */
  .tc-svg {
    width: 100%;
    height: 100%;
  }

  /* Ring draw — stroke-dasharray matches circumference: 2π×36 ≈ 226 */
  .tc-ring {
    stroke-dasharray: 226;
    stroke-dashoffset: 226;
    transition: stroke-dashoffset 480ms cubic-bezier(0.4, 0, 0.2, 1) 0ms;
  }

  .tc-ring--draw {
    stroke-dashoffset: 0;
  }

  /* Checkmark draw — rough path length ≈ 58 */
  .tc-check {
    stroke-dasharray: 58;
    stroke-dashoffset: 58;
    transition: stroke-dashoffset 300ms cubic-bezier(0.4, 0, 0.2, 1) 360ms;
  }

  .tc-check--draw {
    stroke-dashoffset: 0;
  }

  /* ── Text ────────────────────────────────────────────────────────── */
  .tc-text {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    transform: translateY(14px);
    opacity: 0;
    transition:
      transform 300ms cubic-bezier(0.22, 1, 0.36, 1) 0ms,
      opacity   250ms ease-out 0ms;
  }

  .tc-text--in {
    transform: translateY(0);
    opacity: 1;
  }

  .tc-title {
    margin: 0;
    font-family: var(--nb-font-display, 'Poppins', sans-serif);
    font-weight: 700;
    font-size: 1.35rem;
    color: var(--nb-text, #0A0A0A);
    letter-spacing: -0.02em;
    text-align: center;
  }

  /* Badge row */
  .tc-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 5px 14px;
    background: #F0FFF8;
    border: 2px solid #00C875;
    font-family: var(--nb-font-body, var(--nb-font-display), sans-serif);
    font-size: 0.82rem;
    font-weight: 600;
    color: #00875A;
  }

  .tc-badge__dot {
    color: #00C875;
    font-weight: 400;
  }

  .tc-badge__size {
    font-family: var(--nb-font-mono);
    font-size: 0.78rem;
    color: #00875A;
  }

  .tc-badge__count {
    font-family: var(--nb-font-mono);
    font-size: 0.82rem;
  }

  /* ── Reduced motion ─────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .tc-ripple--1,
    .tc-ripple--2    { animation: none; }
    .tc-card         { transition: opacity 150ms; transform: none; }
    .tc-card--out    { transition: opacity 150ms; transform: none; }
    .tc-circle       { transition: opacity 150ms; transform: scale(1); }
    .tc-ring         { transition: none; stroke-dashoffset: 0; }
    .tc-check        { transition: none; stroke-dashoffset: 0; }
    .tc-text         { transition: opacity 150ms; transform: none; }
  }
</style>
