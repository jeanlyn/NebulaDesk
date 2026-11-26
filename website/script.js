/* NebulaDesk site — constellation canvas + reveals */
(function () {
  'use strict';

  /* ---- starfield / graph constellation ---- */
  var canvas = document.getElementById('stars');
  var reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  if (canvas && !reduced) {
    var ctx = canvas.getContext('2d');
    var nodes = [];
    var N = 70;
    var LINK = 130;
    var w, h, dpr;

    function resize() {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      w = canvas.width = Math.floor(innerWidth * dpr);
      h = canvas.height = Math.floor(innerHeight * dpr);
      canvas.style.width = innerWidth + 'px';
      canvas.style.height = innerHeight + 'px';
    }
    resize();
    window.addEventListener('resize', resize);

    for (var i = 0; i < N; i++) {
      nodes.push({
        x: Math.random(), y: Math.random(),
        vx: (Math.random() - 0.5) * 0.00035,
        vy: (Math.random() - 0.5) * 0.00035,
        r: 0.8 + Math.random() * 1.6,
        c: Math.random() < 0.75 ? '129,140,248' : '34,211,238'
      });
    }

    function tick() {
      ctx.clearRect(0, 0, w, h);
      var i, j, a, b, dx, dy, d;
      for (i = 0; i < N; i++) {
        a = nodes[i];
        a.x += a.vx; a.y += a.vy;
        if (a.x < 0 || a.x > 1) a.vx *= -1;
        if (a.y < 0 || a.y > 1) a.vy *= -1;
      }
      ctx.lineWidth = dpr * 0.6;
      for (i = 0; i < N; i++) {
        for (j = i + 1; j < N; j++) {
          a = nodes[i]; b = nodes[j];
          dx = (a.x - b.x) * w; dy = (a.y - b.y) * h;
          d = Math.sqrt(dx * dx + dy * dy);
          if (d < LINK * dpr) {
            ctx.strokeStyle = 'rgba(99,102,241,' + (0.10 * (1 - d / (LINK * dpr))).toFixed(3) + ')';
            ctx.beginPath();
            ctx.moveTo(a.x * w, a.y * h);
            ctx.lineTo(b.x * w, b.y * h);
            ctx.stroke();
          }
        }
      }
      for (i = 0; i < N; i++) {
        a = nodes[i];
        ctx.fillStyle = 'rgba(' + a.c + ',0.5)';
        ctx.beginPath();
        ctx.arc(a.x * w, a.y * h, a.r * dpr, 0, 6.2832);
        ctx.fill();
      }
      requestAnimationFrame(tick);
    }
    requestAnimationFrame(tick);
  }

  /* ---- reveal on scroll ---- */
  var els = document.querySelectorAll('.reveal');
  var noanim = /[?&]noanim/.test(location.search);
  if (noanim) {
    document.documentElement.classList.add('noanim');
    els.forEach(function (el) { el.classList.add('in'); });
  } else if ('IntersectionObserver' in window) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) {
          e.target.classList.add('in');
          io.unobserve(e.target);
        }
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -6% 0px' });
    els.forEach(function (el) { io.observe(el); });
  } else {
    els.forEach(function (el) { el.classList.add('in'); });
  }

  /* ---- nav shadow ---- */
  var nav = document.getElementById('nav');
  function onScroll() {
    if (window.scrollY > 14) nav.classList.add('scrolled');
    else nav.classList.remove('scrolled');
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();
})();
