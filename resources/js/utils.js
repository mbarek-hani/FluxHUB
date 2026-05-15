/**
 * utils.js — Shared utility functions used across all Alpine components.
 */

/**
 * Show a toast notification.
 * @param {string} message
 * @param {"success"|"error"|"info"} type
 */
export function showToast(message, type) {
  const container = document.getElementById("toasts");
  if (!container) return;
  const toast = document.createElement("div");
  toast.className = "toast toast-" + type;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => toast.remove(), 4000);
}

/**
 * Wrapper around fetch with JSON content-type and credentials.
 * @param {string} url
 * @param {RequestInit} [options]
 * @returns {Promise<Response>}
 */
export async function apiFetch(url, options = {}) {
  const defaults = {
    headers: { "Content-Type": "application/json" },
    credentials: "same-origin",
  };
  return fetch(url, {
    ...defaults,
    ...options,
    headers: { ...defaults.headers, ...(options.headers || {}) },
  });
}

/**
 * Escape HTML special characters to prevent XSS in innerHTML.
 * @param {string} text
 * @returns {string}
 */
export function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Format a byte count to a human-readable string (B / KB / MB).
 * @param {number} bytes
 * @returns {string}
 */
export function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

/**
 * Map a file extension to a highlight.js language name.
 * @param {string} filename
 * @returns {string}
 */
export function getLanguageForFile(filename) {
  if (filename.endsWith(".blade.php")) return "php";
  const ext = filename.split(".").pop().toLowerCase();
  const map = {
    php: "php",
    js: "javascript",
    json: "json",
    css: "css",
    html: "xml",
    xml: "xml",
    md: "markdown",
    sql: "sql",
    sh: "bash",
    bash: "bash",
    yml: "yaml",
    yaml: "yaml",
    env: "bash",
    txt: "plaintext",
  };
  return map[ext] || "plaintext";
}

/**
 * Return an SVG string for a file/folder icon.
 * @param {string} name   - File or folder name
 * @param {boolean} isDir
 * @param {boolean} [isOpen] - Only relevant when isDir is true
 * @returns {string}
 */
export function getFileIcon(name, isDir, isOpen = false) {
  if (isDir) {
    return isOpen
      ? '<svg class="icon-folder-open" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>'
      : '<svg class="icon-folder" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M10 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-8l-2-2z"/></svg>';
  }

  const ext = name.split(".").pop().toLowerCase();
  let colorClass = "";
  if (name.endsWith(".blade.php"))  colorClass = "icon-blade";
  else if (ext === "php")           colorClass = "icon-php";
  else if (ext === "json")          colorClass = "icon-json";
  else if (ext === "md")            colorClass = "icon-md";
  else if (ext === "js")            colorClass = "icon-js";
  else if (ext === "css")           colorClass = "icon-css";

  return (
    `<svg class="${colorClass}" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">` +
    `<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>` +
    `<polyline points="14 2 14 8 20 8"/></svg>`
  );
}
