// Pequenos aprimoramentos progressivos da interface. Nada aqui é essencial
// para a navegação: os formulários e links funcionam sem JavaScript.

// Assinatura no console, no mesmo tom do tema.
try {
  console.log(
    "%c>_ REBELLION%c recon database\n   interface somente leitura · postgresql",
    "color:#4ade80;background:#0b0f0e;font:600 12px monospace;padding:2px 4px",
    "color:#7d8f86;font:12px monospace",
  );
} catch {
  // Console indisponível: segue o jogo.
}

document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-copy]");
  if (!button) {
    return;
  }

  const value = button.getAttribute("data-copy");
  if (!value) {
    return;
  }

  try {
    await copyText(value);
    const original = button.textContent;
    button.textContent = "copiado";
    button.classList.add("copied");
    setTimeout(() => {
      button.textContent = original;
      button.classList.remove("copied");
    }, 1400);
  } catch {
    button.textContent = "falhou";
    setTimeout(() => {
      button.textContent = "copiar";
    }, 1400);
  }
});

async function copyText(value) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!ok) {
    throw new Error("copy failed");
  }
}

// Atalho "/" para focar a busca da página.
document.addEventListener("keydown", (event) => {
  if (event.key !== "/" || event.metaKey || event.ctrlKey || event.altKey) {
    return;
  }

  const tag = document.activeElement ? document.activeElement.tagName : "";
  if (/^(INPUT|TEXTAREA|SELECT)$/.test(tag)) {
    return;
  }

  const search = document.querySelector('input[type="search"]');
  if (search) {
    event.preventDefault();
    search.focus();
  }
});
