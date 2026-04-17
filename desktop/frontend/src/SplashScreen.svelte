<script>
  import { onMount, createEventDispatcher } from "svelte";
  import logoImg from "./assets/images/icon.png";

  const dispatch = createEventDispatcher();

  let phase = "idle"; 
  let showMarquee = false;
  
  // High-end metadata labels
  const metaLeft = "[ P2P_ENCRYPTED_V2.3 ]";
  const metaRight = "[ SYNC_NODE_READY ]";

  const marqueeText = Array(12).fill("BEAMSYNC");

  onMount(() => {
    const prefersReduced = window.matchMedia(
      "(prefers-reduced-motion: reduce)"
    ).matches;
    
    if (prefersReduced) {
      setTimeout(() => dispatch("done"), 300);
      return;
    }

    // Sequence for "Component Assembly"
    setTimeout(() => { phase = "assembling"; }, 50);
    setTimeout(() => { showMarquee = true; }, 300);
    
    // Hold for impact
    setTimeout(() => {
      phase = "door-open";
    }, 2400);

    // After doors open, dispatch done
    setTimeout(() => dispatch("done"), 3000);
  });
</script>

<div
  class="splash-sys nb-root"
  role="status"
  aria-live="polite"
  class:splash-sys--exit={phase === "door-open"}
>
  <!-- SVG NOISE FILTER -->
  <svg style="display:none">
    <filter id="grainy-noise">
      <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="3" stitchTiles="stitch" />
      <feColorMatrix type="saturate" values="0" />
      <feComponentTransfer>
        <feFuncR type="linear" slope="0.15" />
        <feFuncG type="linear" slope="0.15" />
        <feFuncB type="linear" slope="0.15" />
      </feComponentTransfer>
    </filter>
  </svg>

  <!-- BACKGROUND DOORS -->
  <div class="door door--top">
    <div class="marquee-tape marquee-tape--yellow" class:active={showMarquee}>
      <div class="marquee-track">
        {#each marqueeText as text}
          <span>{text}</span><span class="star">★</span>
        {/each}
        {#each marqueeText as text}
          <span>{text}</span><span class="star">★</span>
        {/each}
      </div>
    </div>
  </div>
  
  <div class="door door--bottom">
    <div class="marquee-tape marquee-tape--blue" class:active={showMarquee}>
      <div class="marquee-track marquee-track--reverse">
        {#each marqueeText as text}
          <span>{text}</span><span class="star">★</span>
        {/each}
        {#each marqueeText as text}
          <span>{text}</span><span class="star">★</span>
        {/each}
      </div>
    </div>
  </div>

  <!-- CENTER STAGE: THE PREMIUM SYNC MODULE -->
  <div class="center-stage" class:center-stage--exit={phase === "door-open"}>
    
    <!-- Machined Border Container -->
    <div class="module-border-outer">
      <div class="module-border-inner">
        
        <div class="brutal-card">
          <!-- Tactile Grain Overlay -->
          <div class="grain-overlay"></div>
          
          <!-- Corner Metadata Labels -->
          <div class="meta-label meta-label--tl">{metaLeft}</div>
          <div class="meta-label meta-label--tr">{metaRight}</div>
          <div class="meta-label meta-label--bl">0x{Math.random().toString(16).substr(2, 6).toUpperCase()}</div>
          <div class="meta-label meta-label--br">REV_801</div>

          <div class="card-inner">
            <!-- Left Panel: Logo (Snaps in from left) -->
            <div class="logo-area" class:logo-area--in={phase !== "idle"}>
              <div class="logo-glow"></div>
              <img src={logoImg} alt="BeamSync" class="logo-image" draggable="false" />
            </div>

            <!-- Right Panel: Text (Snaps in from right) -->
            <div class="text-area" class:text-area--in={phase !== "idle"}>
              <div class="brand-group">
                <h1 class="brand-title">BEAMSYNC</h1>
                <div class="title-glitch" aria-hidden="true">BEAMSYNC</div>
              </div>
              <div class="divider-track">
                <div class="divider-fill" class:divider-fill--go={phase !== "idle"}></div>
              </div>
              <p class="brand-sub">LOCAL PEER-TO-PEER DATA_TRANSFER_PROTOCOL</p>
            </div>
          </div>

          <!-- Bottom Progress Module -->
          <div class="brutal-progress">
            <div class="progress-info">
              <span class="progress-status">SYNC_IN_PROGRESS</span>
              <span class="progress-percent">0..100</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill"></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  /* ─── BASE SPLASH OVERLAY ──────────────────────────────────────────────── */
  .splash-sys {
    position: fixed;
    inset: 0;
    z-index: 99999;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    background: #000; /* Dark background between splits */
  }

  /* ─── ELEVATOR DOORS ───────────────────────────────────────────────────── */
  .door {
    position: absolute;
    left: 0;
    right: 0;
    height: 50vh;
    background: var(--nb-bg, #F5F0E8);
    transition: transform 700ms cubic-bezier(0.8, 0, 0.2, 1);
    z-index: 1;
    overflow: hidden;
  }

  .door--top {
    top: 0;
    border-bottom: 6px solid var(--nb-border-color, #0A0A0A);
  }

  .door--bottom {
    bottom: 0;
    border-top: 6px solid var(--nb-border-color, #0A0A0A);
  }

  .splash-sys--exit .door--top { transform: translateY(-100%); }
  .splash-sys--exit .door--bottom { transform: translateY(100%); }

  /* ─── MARQUEE TAPES ────────────────────────────────────────────────────── */
  .marquee-tape {
    position: absolute;
    width: 140vw;
    left: -20vw;
    padding: 16px 0;
    border-top: 4px solid var(--nb-border-color, #0A0A0A);
    border-bottom: 4px solid var(--nb-border-color, #0A0A0A);
    transform: rotate(-2deg);
    white-space: nowrap;
    opacity: 0;
    transition: transform 600ms cubic-bezier(0.175, 0.885, 0.32, 1.275), opacity 300ms ease;
  }

  .door--bottom .marquee-tape { rotate: 2deg; transform: rotate(2deg) scaleX(0); }
  .door--top .marquee-tape { transform: rotate(-2deg) scaleX(0); }

  .marquee-tape.active { opacity: 1; transform: rotate(-2deg) scaleX(1); }
  .door--bottom .marquee-tape.active { transform: rotate(2deg) scaleX(1); }

  .marquee-tape--yellow { background: var(--nb-secondary, #FF7A00); top: 30%; }
  .marquee-tape--blue { background: var(--nb-primary, #1A56FF); bottom: 30%; }

  .marquee-track {
    display: flex;
    gap: 3rem;
    font-family: var(--nb-font-display, "Syne", sans-serif);
    font-size: 2.2rem;
    font-weight: 800;
    color: #0A0A0A;
    animation: scroll-left 15s linear infinite;
  }
  .marquee-track--reverse { animation: scroll-right 15s linear infinite; color: #fff; }

  @keyframes scroll-left { 0% { transform: translateX(0); } 100% { transform: translateX(-50%); } }
  @keyframes scroll-right { 0% { transform: translateX(-50%); } 100% { transform: translateX(0%); } }

  /* ─── CENTER STAGE: PREMIUM MODULE ─────────────────────────────────────── */
  .center-stage {
    position: relative;
    z-index: 10;
    filter: drop-shadow(0 20px 40px rgba(0,0,0,0.3));
  }

  .center-stage--exit {
    transition: transform 400ms cubic-bezier(0.8, 0, 0.2, 1), opacity 300ms ease;
    transform: scale(0.9) rotate(2deg);
    opacity: 0;
  }

  /* Machined border stack */
  .module-border-outer {
    padding: 8px;
    background: var(--nb-border-color, #0A0A0A);
    border: 2px solid rgba(255,255,255,0.1);
  }

  .module-border-inner {
    padding: 4px;
    border: 2px solid var(--nb-bg, #F5F0E8);
  }

  .brutal-card {
    background: var(--nb-surface, #fff);
    border: 6px solid var(--nb-border-color, #0A0A0A);
    box-shadow: 16px 16px 0px var(--nb-border-color, #0A0A0A);
    display: flex;
    flex-direction: column;
    position: relative;
    overflow: hidden;
  }

  /* Tactile Grain Overlay */
  .grain-overlay {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 20;
    opacity: 0.4;
    filter: url(#grainy-noise);
  }

  /* Metadata Labels */
  .meta-label {
    position: absolute;
    font-family: var(--nb-font-mono, monospace);
    font-size: 0.55rem;
    font-weight: 800;
    color: var(--nb-text-muted, #4A4A4A);
    z-index: 25;
    padding: 4px 8px;
    letter-spacing: 0.05em;
  }
  .meta-label--tl { top: 4px; left: 4px; }
  .meta-label--tr { top: 4px; right: 4px; }
  .meta-label--bl { bottom: 30px; left: 4px; }
  .meta-label--br { bottom: 30px; right: 4px; }

  .card-inner { display: flex; align-items: stretch; background: #0A0A0A; }

  /* Left Panel: Logo Case */
  .logo-area {
    background: var(--nb-primary, #1A56FF);
    padding: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-right: 6px solid #0A0A0A;
    position: relative;
    /* Snap in from left */
    transform: translateX(-100%);
    transition: transform 500ms cubic-bezier(0.2, 1, 0.3, 1) 100ms;
  }
  .logo-area--in { transform: translateX(0); }

  .logo-glow {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle, rgba(255,255,255,0.3) 0%, transparent 70%);
    opacity: 0.5;
  }

  .logo-image {
    width: 80px;
    height: 80px;
    object-fit: contain;
    z-index: 2;
    filter: drop-shadow(6px 6px 0 rgba(0,0,0,0.5));
    animation: logo-throb 3s infinite ease-in-out;
  }

  @keyframes logo-throb {
    0%, 100% { transform: scale(1) rotate(0deg); }
    50% { transform: scale(1.08) rotate(3deg); }
  }

  /* Right Panel: Content */
  .text-area {
    background: #141414; /* Specific deep black for tech look */
    padding: 40px 60px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    /* Snap in from right */
    transform: translateX(100%);
    transition: transform 500ms cubic-bezier(0.2, 1, 0.3, 1) 150ms;
  }
  .text-area--in { transform: translateX(0); }

  .brand-group { position: relative; }
  
  .brand-title {
    font-family: var(--nb-font-display, "Syne", sans-serif);
    font-size: 4rem;
    font-weight: 800;
    line-height: 0.9;
    color: #fff;
    letter-spacing: -0.04em;
    margin: 0;
  }

  .title-glitch {
    position: absolute;
    top: 0; left: 0;
    color: var(--nb-secondary, #FF7A00);
    opacity: 0.2;
    transform: translate(4px, 4px);
    font-family: var(--nb-font-display, "Syne", sans-serif);
    font-size: 4rem;
    font-weight: 800;
    line-height: 0.9;
    letter-spacing: -0.04em;
    pointer-events: none;
  }

  .divider-track {
    height: 8px;
    width: 60px;
    background: rgba(255,255,255,0.1);
    margin: 20px 0;
    overflow: hidden;
  }
  .divider-fill {
    height: 100%;
    width: 0%;
    background: var(--nb-secondary, #FF7A00);
    transition: width 800ms ease 600ms;
  }
  .divider-fill--go { width: 100%; }

  .brand-sub {
    font-family: var(--nb-font-mono, "Space Mono", monospace);
    font-size: 0.75rem;
    font-weight: 700;
    color: rgba(255,255,255,0.5);
    margin: 0;
    letter-spacing: 0.15em;
    text-transform: uppercase;
  }

  /* Progress Module */
  .brutal-progress {
    background: var(--nb-surface, #fff);
    border-top: 6px solid #0A0A0A;
    padding: 12px 20px;
  }

  .progress-info {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .progress-status { font-family: var(--nb-font-mono, monospace); font-size: 0.6rem; font-weight: 800; }
  .progress-percent { font-family: var(--nb-font-mono, monospace); font-size: 0.6rem; font-weight: 800; color: var(--nb-primary, #1A56FF); }

  .progress-track { height: 8px; background: #EEE; border: 2px solid #0A0A0A; overflow: hidden; }
  .progress-fill {
    height: 100%;
    width: 0%;
    background: var(--nb-secondary, #FF7A00);
    border-right: 3px solid #0A0A0A;
    animation: assembly-fill 1.8s cubic-bezier(0.8, 0, 0.2, 1) forwards 800ms;
  }

  @keyframes assembly-fill { 0% { width: 0%; } 40% { width: 40%; } 70% { width: 40%; } 100% { width: 100%; } }

  /* ─── DARK MODE ADAPTATION ──────────────────────────────────────────────── */
  @media (prefers-color-scheme: dark) {
    .door { background: #0C1120; border-color: #2D3F5E; }
    .marquee-tape { border-color: #2D3F5E; }
    .marquee-track { color: #fff; }
    .marquee-tape--yellow .marquee-track { color: #000; }
    
    .module-border-outer { background: #2D3F5E; }
    .module-border-inner { border-color: #0C1120; }
    
    .brutal-card { border-color: #2D3F5E; box-shadow: 16px 16px 0px #08101E; background: #141E30; }
    .logo-area { border-color: #2D3F5E; }
    .text-area { background: #080D1A; }
    .title-glitch { color: var(--nb-primary); opacity: 0.3; }
    
    .brutal-progress { background: #141E30; border-color: #2D3F5E; }
    .progress-track { background: #080D1A; border-color: #2D3F5E; }
    .progress-fill { border-color: #2D3F5E; }
    .progress-status { color: #E2E8F0; }
  }

  @media (max-width: 800px) {
    .brand-title { font-size: 2.5rem; }
    .title-glitch { font-size: 2.5rem; }
    .text-area { padding: 30px 40px; }
  }
</style>
