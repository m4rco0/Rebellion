// Grafo de domínios e subdomínios. Sem dependências: layout de forças simples,
// SVG e interações básicas de arrastar, ampliar e clicar. Se o JavaScript
// falhar, a página continua oferecendo a lista de domínios.
(() => {
  "use strict";

  const canvas = document.getElementById("graph");
  if (!canvas || !canvas.dataset.src) {
    return;
  }

  const status = document.getElementById("graph-status");
  const NS = "http://www.w3.org/2000/svg";
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  fetch(canvas.dataset.src, { headers: { Accept: "application/json" } })
    .then((response) => {
      if (!response.ok) {
        throw new Error("HTTP " + response.status);
      }
      return response.json();
    })
    .then(render)
    .catch((error) => {
      canvas.replaceChildren(fallbackMessage());
      if (status) {
        status.textContent = "erro ao carregar";
      }
      console.error("grafo:", error);
    });

  function fallbackMessage() {
    const paragraph = document.createElement("p");
    paragraph.className = "graph-fallback";
    paragraph.append("Não foi possível carregar o grafo. ");
    const link = document.createElement("a");
    link.href = "/domains";
    link.textContent = "Ver domínios em lista";
    paragraph.append(link);
    paragraph.append(".");
    return paragraph;
  }

  function render(data) {
    const nodes = (data.nodes || []).map((node) => ({
      ...node,
      x: 0,
      y: 0,
      vx: 0,
      vy: 0,
    }));

    if (nodes.length === 0) {
      canvas.replaceChildren(fallbackMessage());
      if (status) {
        status.textContent = "nenhum nó para exibir";
      }
      return;
    }

    const byID = new Map(nodes.map((node) => [node.id, node]));
    const links = (data.links || [])
      .map((link) => ({
        source: byID.get(link.source),
        target: byID.get(link.target),
      }))
      .filter((link) => link.source && link.target);

    const width = Math.max(320, canvas.clientWidth || 800);
    const height = Math.max(
      420,
      Math.min(720, Math.round(width * (width < 720 ? 0.9 : 0.58))),
    );
    canvas.style.height = height + "px";

    const svg = make("svg", {
      viewBox: `0 0 ${width} ${height}`,
      role: "img",
      "aria-label": "Grafo de domínios e subdomínios",
    });
    const viewport = make("g");
    svg.appendChild(viewport);

    const view = { x: 0, y: 0, k: 1 };
    const padding = 34;
    let animationToken = 0;

    const radiusOf = (node) => {
      if (node.type === "domain") {
        return 11 + Math.min(8, Math.sqrt(Math.max(0, node.count || 0)));
      }
      return 5 + Math.min(3, Math.log2(Math.max(0, node.count || 0) + 1));
    };

    const showSubdomainLabels = nodes.length <= 60;

    const linkEls = links.map((link) => {
      const line = make("line", { class: "graph-link" });
      viewport.appendChild(line);
      return line;
    });

    const nodeEls = nodes.map((node) => {
      const group = make("g");
      const circle = make("circle", {
        class: "graph-node " + (node.type === "domain" ? "domain" : "subdomain"),
        r: radiusOf(node),
        tabindex: "0",
        role: "link",
        "aria-label": node.label,
      });
      const label = make("text", {
        class:
          "graph-label " +
          (node.type === "domain" ? "domain" : "") +
          (showSubdomainLabels || node.type === "domain" ? "" : " hidden"),
      });
      label.textContent = shortLabel(node.label);
      group.append(circle, label);
      viewport.appendChild(group);
      return { node, group, circle, label };
    });

    canvas.replaceChildren(svg);

    const tooltip = document.createElement("div");
    tooltip.className = "graph-tooltip";
    tooltip.hidden = true;
    document.body.appendChild(tooltip);

    layout();
    if (reduceMotion) {
      for (let i = 0; i < 260; i += 1) {
        step(0.9);
      }
      draw();
    } else {
      animate();
    }
    updateStatus();

    // ------------------------------------------------------------------ layout

    function layout() {
      const domains = nodes.filter((node) => node.type === "domain");
      const centerX = width / 2;
      const centerY = height / 2;

      domains.forEach((domain, index) => {
        if (domains.length === 1) {
          domain.x = centerX;
          domain.y = centerY;
          return;
        }
        const angle = (2 * Math.PI * index) / domains.length - Math.PI / 2;
        const rx = Math.min(width * 0.3, 260);
        const ry = Math.min(height * 0.3, 220);
        domain.x = centerX + Math.cos(angle) * rx;
        domain.y = centerY + Math.sin(angle) * ry;
      });

      const children = new Map(domains.map((domain) => [domain.id, []]));
      links.forEach((link) => {
        const bucket = children.get(link.source.id);
        if (bucket) {
          bucket.push(link.target);
        }
      });

      children.forEach((bucket, domainID) => {
        const domain = byID.get(domainID);
        const ring = 46 + Math.min(90, bucket.length * 3.2);
        bucket.forEach((child, index) => {
          const angle = (2 * Math.PI * index) / bucket.length;
          const wobble = ((index % 5) - 2) * 5;
          child.x = domain.x + Math.cos(angle) * (ring + wobble);
          child.y = domain.y + Math.sin(angle) * (ring + wobble);
        });
      });
    }

    function step(alpha) {
      const repulsion = 1700;
      const spring = 0.022;
      const ideal = 72;
      const gravity = 0.0035;
      const centerX = width / 2;
      const centerY = height / 2;

      for (let i = 0; i < nodes.length; i += 1) {
        const a = nodes[i];
        for (let j = i + 1; j < nodes.length; j += 1) {
          const b = nodes[j];
          let dx = a.x - b.x;
          let dy = a.y - b.y;
          let dist2 = dx * dx + dy * dy;
          if (dist2 < 0.01) {
            dx = (i % 7) - 3 || 1;
            dy = (j % 5) - 2 || 1;
            dist2 = dx * dx + dy * dy;
          }
          if (dist2 > 90000) {
            continue;
          }
          const dist = Math.sqrt(dist2);
          const boost =
            a.type === "domain" || b.type === "domain" ? 2.2 : 1;
          const force = (repulsion * boost * alpha) / dist2;
          const fx = (dx / dist) * force;
          const fy = (dy / dist) * force;
          a.vx += fx;
          a.vy += fy;
          b.vx -= fx;
          b.vy -= fy;
        }
      }

      links.forEach((link) => {
        const dx = link.target.x - link.source.x;
        const dy = link.target.y - link.source.y;
        const dist = Math.sqrt(dx * dx + dy * dy) || 0.01;
        const force = (dist - ideal) * spring * alpha;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;
        link.source.vx += fx;
        link.source.vy += fy;
        link.target.vx -= fx;
        link.target.vy -= fy;
      });

      nodes.forEach((node) => {
        node.vx += (centerX - node.x) * gravity * alpha;
        node.vy += (centerY - node.y) * gravity * alpha;
        node.vx *= 0.84;
        node.vy *= 0.84;
        node.x += node.vx;
        node.y += node.vy;
        node.x = Math.max(padding, Math.min(width - padding, node.x));
        node.y = Math.max(padding, Math.min(height - padding, node.y));
      });
    }

    function draw() {
      viewport.setAttribute(
        "transform",
        `translate(${view.x} ${view.y}) scale(${view.k})`,
      );

      links.forEach((link, index) => {
        const line = linkEls[index];
        line.setAttribute("x1", link.source.x);
        line.setAttribute("y1", link.source.y);
        line.setAttribute("x2", link.target.x);
        line.setAttribute("y2", link.target.y);
      });

      nodeEls.forEach(({ node, group, label }) => {
        group.setAttribute("transform", `translate(${node.x} ${node.y})`);
        label.setAttribute("x", 0);
        label.setAttribute("y", radiusOf(node) + 13);
        label.setAttribute("text-anchor", "middle");
      });
    }

    function animate() {
      animationToken += 1;
      const token = animationToken;
      let alpha = 1;
      const tick = () => {
        if (token !== animationToken) {
          return;
        }
        step(alpha);
        draw();
        alpha *= 0.985;
        if (alpha > 0.03) {
          requestAnimationFrame(tick);
        }
      };
      requestAnimationFrame(tick);
    }

    function updateStatus() {
      if (!status) {
        return;
      }
      const domainCount = nodes.filter((node) => node.type === "domain").length;
      const subdomainCount = nodes.length - domainCount;
      let text = `${domainCount} domínio(s) · ${subdomainCount} subdomínio(s)`;
      if (data.truncated) {
        text += ` de ${data.total}`;
      }
      if (data.domains_truncated) {
        text += " · alguns domínios ocultos";
      }
      status.textContent = text;
    }

    // ------------------------------------------------------------ interações

    const rectPoint = (event) => {
      const rect = svg.getBoundingClientRect();
      return {
        x: ((event.clientX - rect.left) * width) / rect.width,
        y: ((event.clientY - rect.top) * height) / rect.height,
      };
    };

    const toGraph = (event) => {
      const point = rectPoint(event);
      return {
        x: (point.x - view.x) / view.k,
        y: (point.y - view.y) / view.k,
      };
    };

    let dragging = null;
    let panning = null;
    let moved = 0;

    const reheat = () => {
      if (reduceMotion) {
        for (let i = 0; i < 140; i += 1) {
          step(0.8);
        }
        draw();
      } else {
        animate();
      }
    };

    svg.addEventListener("pointerdown", (event) => {
      if (event.target === svg || event.target === viewport) {
        panning = {
          startX: event.clientX,
          startY: event.clientY,
          viewX: view.x,
          viewY: view.y,
        };
        svg.classList.add("dragging");
        svg.setPointerCapture(event.pointerId);
      }
    });

    nodeEls.forEach(({ node, circle }) => {
      circle.addEventListener("pointerdown", (event) => {
        event.stopPropagation();
        const point = toGraph(event);
        dragging = {
          node,
          dx: node.x - point.x,
          dy: node.y - point.y,
        };
        moved = 0;
        svg.setPointerCapture(event.pointerId);
      });

      circle.addEventListener("pointerenter", (event) => {
        showTooltip(node, event);
        highlight(node, true);
      });

      circle.addEventListener("pointermove", (event) => {
        positionTooltip(event);
      });

      circle.addEventListener("pointerleave", () => {
        hideTooltip();
        highlight(node, false);
      });

      circle.addEventListener("focus", () => {
        highlight(node, true);
      });

      circle.addEventListener("blur", () => {
        highlight(node, false);
      });

      circle.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          navigate(node);
        }
      });

      circle.addEventListener("click", (event) => {
        event.stopPropagation();
        if (moved < 5) {
          navigate(node);
        }
      });
    });

    svg.addEventListener("pointermove", (event) => {
      if (dragging) {
        const point = toGraph(event);
        moved += Math.abs(event.movementX) + Math.abs(event.movementY);
        dragging.node.x = point.x + dragging.dx;
        dragging.node.y = point.y + dragging.dy;
        dragging.node.vx = 0;
        dragging.node.vy = 0;
        draw();
        return;
      }

      if (panning) {
        view.x = panning.viewX + (event.clientX - panning.startX);
        view.y = panning.viewY + (event.clientY - panning.startY);
        draw();
      }
    });

    const endPointer = () => {
      if (dragging) {
        dragging = null;
        reheat();
      }
      panning = null;
      svg.classList.remove("dragging");
    };

    svg.addEventListener("pointerup", endPointer);
    svg.addEventListener("pointercancel", endPointer);

    svg.addEventListener(
      "wheel",
      (event) => {
        event.preventDefault();
        const factor = Math.exp(-event.deltaY * 0.0015);
        const next = Math.max(0.4, Math.min(3, view.k * factor));
        const point = rectPoint(event);
        view.x = point.x - ((point.x - view.x) * next) / view.k;
        view.y = point.y - ((point.y - view.y) * next) / view.k;
        view.k = next;
        draw();
      },
      { passive: false },
    );

    function highlight(node, active) {
      const connected = new Set([node.id]);
      links.forEach((link) => {
        if (link.source.id === node.id) {
          connected.add(link.target.id);
        }
        if (link.target.id === node.id) {
          connected.add(link.source.id);
        }
      });

      links.forEach((link, index) => {
        linkEls[index].classList.toggle(
          "active",
          active && connected.has(link.source.id) && connected.has(link.target.id),
        );
      });

      nodeEls.forEach(({ node: other, label }) => {
        if (active && connected.has(other.id)) {
          label.classList.remove("hidden");
        } else if (other.type === "subdomain" && !showSubdomainLabels) {
          label.classList.add("hidden");
        }
      });
    }

    function navigate(node) {
      if (node.url) {
        window.location.href = node.url;
      }
    }

    function showTooltip(node, event) {
      tooltip.replaceChildren();
      const title = document.createElement("strong");
      title.textContent = node.label;
      const detail = document.createElement("small");
      if (node.type === "domain") {
        detail.textContent = `domínio · ${node.count} subdomínio(s)`;
      } else {
        detail.textContent = `subdomínio de ${node.domain} · ${node.count} URL(s)`;
      }
      tooltip.append(title, detail);
      tooltip.hidden = false;
      positionTooltip(event);
    }

    function positionTooltip(event) {
      if (tooltip.hidden) {
        return;
      }
      const margin = 14;
      const rect = tooltip.getBoundingClientRect();
      let x = event.clientX + 16;
      let y = event.clientY + 16;
      if (x + rect.width + margin > window.innerWidth) {
        x = event.clientX - rect.width - 16;
      }
      if (y + rect.height + margin > window.innerHeight) {
        y = event.clientY - rect.height - 16;
      }
      tooltip.style.left = Math.max(margin, x) + "px";
      tooltip.style.top = Math.max(margin, y) + "px";
    }

    function hideTooltip() {
      tooltip.hidden = true;
    }
  }

  function shortLabel(label) {
    if (label.length <= 32) {
      return label;
    }
    return label.slice(0, 15) + "…" + label.slice(-14);
  }

  function make(tag, attributes = {}) {
    const element = document.createElementNS(NS, tag);
    Object.entries(attributes).forEach(([name, value]) => {
      element.setAttribute(name, String(value));
    });
    return element;
  }
})();
