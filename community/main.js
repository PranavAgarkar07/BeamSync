/* ============================================================
   BeamSync Community — main.js
   Features: Scroll reveal · Navbar scroll · Terminal animation
             Copy buttons · Stat counters
   ============================================================ */

// ── NAVBAR SCROLL ──────────────────────────────────────────
const navbar = document.getElementById('navbar');
window.addEventListener('scroll', () => {
  navbar.classList.toggle('scrolled', window.scrollY > 40);
}, { passive: true });

// ── SCROLL REVEAL ──────────────────────────────────────────
const revealObserver = new IntersectionObserver((entries) => {
  entries.forEach((entry, i) => {
    if (entry.isIntersecting) {
      // stagger siblings
      const siblings = entry.target.parentElement.querySelectorAll('.reveal');
      let delay = 0;
      siblings.forEach((el, idx) => {
        if (el === entry.target) delay = idx * 80;
      });
      setTimeout(() => {
        entry.target.classList.add('visible');
      }, delay);
      revealObserver.unobserve(entry.target);
    }
  });
}, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });

document.querySelectorAll('.reveal').forEach(el => revealObserver.observe(el));

// ── TERMINAL ANIMATION ─────────────────────────────────────
const lines = [
  { id: 't1', cls: 'cmd', text: 'yay -S beamsync-bin' },
  { id: 't2', cls: 'out', text: ':: Resolving dependencies...' },
  { id: 't3', cls: 'out', text: '✓ BeamSync installed. Run: beamsync' },
];

function typeText(el, text, speed = 38) {
  return new Promise(resolve => {
    let i = 0;
    const tick = () => {
      el.textContent = text.slice(0, ++i);
      if (i < text.length) setTimeout(tick, speed);
      else resolve();
    };
    tick();
  });
}

async function runTerminal() {
  const cursor = document.getElementById('tcursor');
  for (const { id, cls, text } of lines) {
    const el = document.getElementById(id);
    el.className = `terminal-line ${cls}`;
    await typeText(el, text, cls === 'cmd' ? 48 : 18);
    await new Promise(r => setTimeout(r, 320));
  }
  if (cursor) cursor.style.display = 'none';
  // loop after pause
  setTimeout(() => {
    lines.forEach(({ id }) => { document.getElementById(id).textContent = ''; });
    if (cursor) cursor.style.display = '';
    runTerminal();
  }, 4200);
}

// Start terminal after hero reveals
const terminalObserver = new IntersectionObserver((entries) => {
  if (entries[0].isIntersecting) {
    runTerminal();
    terminalObserver.disconnect();
  }
}, { threshold: 0.5 });
const termBox = document.getElementById('terminal-box');
if (termBox) terminalObserver.observe(termBox);

// ── STAT COUNTERS ──────────────────────────────────────────
const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

function animateCounter(el) {
  const raw = el.dataset.target;
  const suffix = el.dataset.suffix || '';
  if (prefersReducedMotion) { el.textContent = el.dataset.suffix || raw + suffix; return; }
  const target = parseInt(raw, 10);
  const duration = 1400;
  const start = performance.now();
  const step = (now) => {
    const progress = Math.min((now - start) / duration, 1);
    const eased = 1 - Math.pow(1 - progress, 3); // ease-out cubic
    const current = Math.round(eased * target);
    el.textContent = current + suffix;
    if (progress < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

const counterObserver = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      animateCounter(entry.target);
      counterObserver.unobserve(entry.target);
    }
  });
}, { threshold: 0.5 });

document.querySelectorAll('.stat-num[data-target]').forEach(el => counterObserver.observe(el));

// ── COPY BUTTONS ───────────────────────────────────────────
document.querySelectorAll('.copy-btn').forEach(btn => {
  btn.addEventListener('click', async () => {
    const text = btn.dataset.copy;
    try {
      await navigator.clipboard.writeText(text);
      const orig = btn.innerHTML;
      btn.innerHTML = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#34D399" stroke-width="2.5" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>`;
      btn.style.color = '#34D399';
      setTimeout(() => {
        btn.innerHTML = orig;
        btn.style.color = '';
      }, 1800);
    } catch {
      // fallback: select text in code block
      const code = btn.previousElementSibling;
      if (code) {
        const range = document.createRange();
        range.selectNodeContents(code);
        window.getSelection().removeAllRanges();
        window.getSelection().addRange(range);
      }
    }
  });
});

// ── SMOOTH ACTIVE NAV ──────────────────────────────────────
const sections = document.querySelectorAll('section[id]');
const navLinks = document.querySelectorAll('.nav-links a[href^="#"]');

const sectionObserver = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      navLinks.forEach(a => {
        a.style.color = a.getAttribute('href') === `#${entry.target.id}`
          ? 'var(--primary)' : '';
      });
    }
  });
}, { threshold: 0.4 });

sections.forEach(s => sectionObserver.observe(s));

// ── SCREENSHOT CAROUSEL ────────────────────────────────────
(function () {
  const track = document.getElementById('ss-track');
  const prevBtn = document.getElementById('ss-prev');
  const nextBtn = document.getElementById('ss-next');
  const dots = document.querySelectorAll('#ss-dots .carousel-dot');
  if (!track) return;

  const total = track.children.length;
  let current = 0;
  let autoTimer;

  function goTo(idx) {
    current = (idx + total) % total;
    track.style.transform = `translateX(-${current * 100}%)`;
    dots.forEach((d, i) => d.classList.toggle('active', i === current));
  }

  prevBtn.addEventListener('click', () => { goTo(current - 1); resetAuto(); });
  nextBtn.addEventListener('click', () => { goTo(current + 1); resetAuto(); });
  dots.forEach(d => d.addEventListener('click', () => { goTo(+d.dataset.idx); resetAuto(); }));

  document.addEventListener('keydown', e => {
    if (e.key === 'ArrowLeft')  { goTo(current - 1); resetAuto(); }
    if (e.key === 'ArrowRight') { goTo(current + 1); resetAuto(); }
  });

  let startX = 0;
  track.addEventListener('touchstart', e => { startX = e.touches[0].clientX; }, { passive: true });
  track.addEventListener('touchend', e => {
    const dx = e.changedTouches[0].clientX - startX;
    if (Math.abs(dx) > 40) { goTo(dx < 0 ? current + 1 : current - 1); resetAuto(); }
  });

  function startAuto() { autoTimer = setInterval(() => goTo(current + 1), 4000); }
  function resetAuto() { clearInterval(autoTimer); startAuto(); }
  track.closest('.carousel').addEventListener('mouseenter', () => clearInterval(autoTimer));
  track.closest('.carousel').addEventListener('mouseleave', startAuto);
  startAuto();
})();

// ── LIGHTBOX ──────────────────────────────────────────────
(function () {
  const slides = [
    { src: 'assets/appSS1.png', alt: 'BeamSync main window — file drop and QR connect', caption: 'File Drop & QR Connect' },
    { src: 'assets/appSS2.png', alt: 'BeamSync live transfer progress',                 caption: 'Live Transfer Progress'  },
    { src: 'assets/appSS3.png', alt: 'BeamSync settings and configuration',              caption: 'Settings & Configuration'},
  ];

  const overlay    = document.getElementById('lb-overlay');
  const lbImg      = document.getElementById('lb-img');
  const lbCaption  = document.getElementById('lb-caption-text');
  const lbClose    = document.getElementById('lb-close');
  const lbPrev     = document.getElementById('lb-prev');
  const lbNext     = document.getElementById('lb-next');
  const lbDotsWrap = document.getElementById('lb-dots');
  if (!overlay) return;

  // Build dots
  const lbDots = slides.map((_, i) => {
    const d = document.createElement('button');
    d.className = 'lb-dot' + (i === 0 ? ' active' : '');
    d.setAttribute('aria-label', `View screenshot ${i + 1}`);
    d.addEventListener('click', () => open(i));
    lbDotsWrap.appendChild(d);
    return d;
  });

  let current = 0;

  function open(idx) {
    current = idx;
    lbImg.src      = slides[idx].src;
    lbImg.alt      = slides[idx].alt;
    lbCaption.textContent = slides[idx].caption;
    lbDots.forEach((d, i) => d.classList.toggle('active', i === idx));
    overlay.classList.add('lb-open');
    document.body.style.overflow = 'hidden';
  }

  function close() {
    overlay.classList.remove('lb-open');
    document.body.style.overflow = '';
  }

  function step(dir) {
    open((current + dir + slides.length) % slides.length);
  }

  // Hook each carousel image wrap
  document.querySelectorAll('.ss-img-wrap').forEach((wrap, i) => {
    wrap.addEventListener('click', () => open(i));
  });

  lbClose.addEventListener('click', close);
  lbPrev.addEventListener('click', () => step(-1));
  lbNext.addEventListener('click', () => step(1));

  // Close on backdrop click
  overlay.addEventListener('click', e => { if (e.target === overlay) close(); });

  // Keyboard
  document.addEventListener('keydown', e => {
    if (!overlay.classList.contains('lb-open')) return;
    if (e.key === 'Escape')      close();
    if (e.key === 'ArrowLeft')   step(-1);
    if (e.key === 'ArrowRight')  step(1);
  });

  // Swipe
  let lbStartX = 0;
  lbImg.addEventListener('touchstart', e => { lbStartX = e.touches[0].clientX; }, { passive: true });
  lbImg.addEventListener('touchend',   e => {
    const dx = e.changedTouches[0].clientX - lbStartX;
    if (Math.abs(dx) > 40) step(dx < 0 ? 1 : -1);
  });
})();

// ── HAMBURGER NAV ─────────────────────────────────────────
(function () {
  const btn    = document.getElementById('nav-hamburger');
  const links  = document.getElementById('nav-links');
  const navBar = document.getElementById('navbar');
  if (!btn || !links || !navBar) return;

  function positionDropdown() {
    const h = navBar.getBoundingClientRect().bottom;
    links.style.top = h + 'px';
  }

  function toggle(force) {
    const open = force !== undefined ? force : !links.classList.contains('open');
    if (open) positionDropdown();
    links.classList.toggle('open', open);
    btn.classList.toggle('open', open);
    btn.setAttribute('aria-expanded', String(open));
    document.body.style.overflow = open ? 'hidden' : '';
  }

  btn.addEventListener('click', (e) => { e.stopPropagation(); toggle(); });

  links.querySelectorAll('a').forEach(a =>
    a.addEventListener('click', () => toggle(false))
  );

  document.addEventListener('click', e => {
    if (!navBar.contains(e.target)) toggle(false);
  });

  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') toggle(false);
  });

  window.addEventListener('resize', () => {
    if (window.innerWidth > 900) toggle(false);
  });
})();

// ── PROFESSIONAL DOWNLOAD BUTTON ─────────────────────────
(function () {
  const dot  = document.getElementById('cursor-dot');
  const ring = document.getElementById('cursor-ring');

  // Detect touch devices — hide custom cursor
  if ('ontouchstart' in window || navigator.maxTouchPoints > 0) {
    document.body.classList.add('touch');
    return;
  }

  // ── Smooth lerped cursor ──────────────────────────────
  // Dot follows mouse immediately; ring lerps with natural lag
  let mouseX = -999, mouseY = -999;
  let ringX  = -999, ringY  = -999;
  let rafId;

  document.addEventListener('mousemove', e => {
    mouseX = e.clientX;
    mouseY = e.clientY;
    if (dot) {
      dot.style.left = mouseX + 'px';
      dot.style.top  = mouseY + 'px';
    }
  }, { passive: true });

  // Ring lerps toward mouse at 12% per frame (natural elastic lag)
  function lerpRing() {
    ringX += (mouseX - ringX) * 0.12;
    ringY += (mouseY - ringY) * 0.12;
    if (ring) {
      ring.style.left = ringX + 'px';
      ring.style.top  = ringY + 'px';
    }
    rafId = requestAnimationFrame(lerpRing);
  }
  // Init ring position on first mousemove
  document.addEventListener('mousemove', function init(e) {
    ringX = e.clientX; ringY = e.clientY;
    lerpRing();
    document.removeEventListener('mousemove', init);
  }, { once: true });

  // ── Button interactions ───────────────────────────────
  document.querySelectorAll('.btn-dl').forEach(btn => {
    // Expand cursor ring on hover
    btn.addEventListener('mouseenter', () => {
      document.body.classList.add('btn-hovered');
    });
    btn.addEventListener('mouseleave', () => {
      document.body.classList.remove('btn-hovered');
    });

    // CSS ripple on click — professional, not gamey
    btn.addEventListener('click', e => {
      if (prefersReducedMotion) return;
      const r      = btn.getBoundingClientRect();
      const size   = Math.max(r.width, r.height) * 2;
      const ripple = document.createElement('span');
      ripple.className = 'ripple';
      ripple.style.cssText = `
        width:${size}px;height:${size}px;
        left:${e.clientX - r.left - size / 2}px;
        top:${e.clientY - r.top  - size / 2}px;
      `;
      btn.appendChild(ripple);
      ripple.addEventListener('animationend', () => ripple.remove());
    });
  });
})();
