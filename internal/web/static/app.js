"use strict";

const input        = document.getElementById("input");
const output       = document.getElementById("output");
const kindEl       = document.getElementById("kind");
const errBox       = document.getElementById("error");
const modesEl      = document.getElementById("modes");
const urlSafeEl    = document.getElementById("urlsafe");
const urlSafeLabel = document.getElementById("urlsafe-label");
const goBtn        = document.getElementById("go");
const copyBtn      = document.getElementById("copy");

// What the primary button says in each mode.
const buttonLabels = {
  "auto":        "Transform",
  "b64-encode":  "Encode",
  "b64-decode":  "Decode",
  "json-pretty": "Pretty-print",
  "json-min":    "Minify",
  "validate":    "Validate",
  "jwt":         "Decode JWT",
};

// Position of the last JSON syntax error, used by click-to-jump.
let lastError = null;

function currentMode() {
  return modesEl.querySelector("input:checked").value;
}

function isB64Mode(mode) {
  return mode === "b64-encode" || mode === "b64-decode";
}

// Keep the controls honest: the button names the action that will
// happen, and the URL-safe toggle is greyed out when it has no effect.
function syncControls() {
  const mode = currentMode();
  goBtn.textContent = buttonLabels[mode];
  const b64 = isB64Mode(mode);
  urlSafeEl.disabled = !b64;
  urlSafeLabel.classList.toggle("muted", !b64);
}

async function transform() {
  errBox.style.display = "none";
  kindEl.style.display = "none";
  kindEl.classList.remove("ok");
  lastError = null;

  const mode = currentMode();

  const res = await fetch("/api/transform", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      input: input.value,
      mode: mode,
      urlSafe: urlSafeEl.checked,
    }),
  });
  const data = await res.json();

  if (!res.ok) {
    output.value = "";
    lastError = data.line ? { line: data.line, column: data.column } : null;
    errBox.textContent = lastError
      ? `${data.error} — click to jump there`
      : data.error;
    errBox.style.display = "block";
    return;
  }

  output.value = data.output;

  if (mode === "auto") {
    kindEl.textContent = "detected: " + data.kind;
    kindEl.style.display = "inline-block";
  } else if (mode === "validate") {
    kindEl.textContent = "✓ " + data.output;
    kindEl.classList.add("ok");
    kindEl.style.display = "inline-block";
  }

  flash(output);
}

function flash(el) {
  el.classList.remove("flash");
  void el.offsetWidth; // force reflow so the animation restarts
  el.classList.add("flash");
}

// Convert a 1-based line/column into a character index usable with
// setSelectionRange. Mirrors lineColumn() on the Go side, inverted.
function positionToIndex(text, line, column) {
  const lines = text.split("\n");
  let idx = 0;
  for (let i = 0; i < line - 1 && i < lines.length; i++) {
    idx += lines[i].length + 1; // +1 for the newline character
  }
  return idx + column - 1;
}

errBox.addEventListener("click", () => {
  if (!lastError) return;
  const idx = positionToIndex(input.value, lastError.line, lastError.column);
  input.focus();
  input.setSelectionRange(idx, idx + 1); // select the offending char
});

async function copyOutput() {
  if (!output.value) return;
  await navigator.clipboard.writeText(output.value);
  copyBtn.textContent = "Copied!";
  setTimeout(() => (copyBtn.textContent = "Copy output"), 1200);
}

goBtn.addEventListener("click", transform);
copyBtn.addEventListener("click", copyOutput);

document.getElementById("swap").addEventListener("click", () => {
  input.value = output.value;
  output.value = "";
  kindEl.style.display = "none";
  errBox.style.display = "none";
  input.focus();
});

// Changing mode or alphabet re-runs the transform on existing input,
// so switching gives instant feedback. One listener on the fieldset
// handles all seven radios (events bubble up from children).
modesEl.addEventListener("change", () => {
  syncControls();
  if (input.value.trim() !== "") transform();
});
urlSafeEl.addEventListener("change", () => {
  if (input.value.trim() !== "") transform();
});

// Transform immediately on paste. The paste event fires BEFORE the
// textarea's value updates, so defer one tick with setTimeout.
input.addEventListener("paste", () => setTimeout(transform, 0));

input.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) transform();
});

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    input.value = "";
    output.value = "";
    errBox.style.display = "none";
    kindEl.style.display = "none";
    input.focus();
  }
  // Alt+C copies. (Ctrl+Shift+C is taken by browser devtools.)
  if (e.altKey && e.key.toLowerCase() === "c") copyOutput();
});

syncControls();
