/**
 * file-browser.js — Alpine.js component for browsing plugin source files.
 *
 * Used on: /admin/plugins/:id/browse
 */
import { apiFetch, showToast, escapeHtml, formatBytes, getFileIcon, getLanguageForFile } from './utils.js';

/**
 * @param {string} pluginID
 * @param {string} initialRef - Git ref to load initially (tag or "HEAD")
 * @param {string[]} tags     - All available tags for the ref selector
 */
export function fileBrowser(pluginID, initialRef, tags) {
  return {
    pluginID,
    currentRef: initialRef || "HEAD",
    tags: tags || [],

    // Tree state
    tree: [],
    flatTree: [],
    expandedDirs: {},
    fileCount: 0,
    loading: false,

    // File viewer state
    currentFile: "",
    fileSize: "",
    fileLoading: false,

    init() {
      this.loadTree();
    },

    // ----------------------------------------------------------------
    // Tree loading
    // ----------------------------------------------------------------

    async loadTree() {
      this.loading = true;
      this.currentFile = "";

      try {
        const resp = await apiFetch(
          `/admin/api/plugins/${this.pluginID}/tree?ref=${encodeURIComponent(this.currentRef)}`
        );
        if (!resp.ok) throw new Error("Failed to load tree");

        this.tree = await resp.json();
        this.flatTree = this._flattenTree(this.tree);
        this.fileCount = this._countFiles(this.tree);

        // Expand top-level directories by default
        this.expandedDirs = {};
        this.tree.forEach((e) => { if (e.is_dir) this.expandedDirs[e.path] = true; });

        this._renderTree();
      } catch (err) {
        showToast("Error loading file tree: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },

    _flattenTree(entries) {
      return entries.reduce((acc, e) => {
        acc.push(e);
        if (e.is_dir && e.children) acc.push(...this._flattenTree(e.children));
        return acc;
      }, []);
    },

    _countFiles(entries) {
      return entries.reduce((n, e) => {
        if (!e.is_dir) n++;
        if (e.children) n += this._countFiles(e.children);
        return n;
      }, 0);
    },

    // ----------------------------------------------------------------
    // Tree rendering
    // ----------------------------------------------------------------

    _renderTree() {
      const container = document.getElementById("file-tree-root");
      if (!container) return;
      container.innerHTML = this._buildTreeHTML(this.tree, 0);
      this._attachTreeListeners(container);
    },

    _buildTreeHTML(entries, depth) {
      const sorted = [...entries].sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        return a.name.localeCompare(b.name);
      });

      return sorted.map((entry) => {
        const paddingLeft = 12 + depth * 16;
        const isExpanded = !!this.expandedDirs[entry.path];

        if (entry.is_dir) {
          const chevron = isExpanded
            ? '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>'
            : '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>';

          const children =
            isExpanded && entry.children?.length
              ? `<div class="tree-children">${this._buildTreeHTML(entry.children, depth + 1)}</div>`
              : "";

          return (
            `<div class="tree-item" data-dir="${escapeHtml(entry.path)}" style="padding-left:${paddingLeft}px">` +
            `<span class="icon" style="width:12px">${chevron}</span>` +
            `<span class="icon">${getFileIcon(entry.name, true, isExpanded)}</span>` +
            `<span>${escapeHtml(entry.name)}</span>` +
            `</div>${children}`
          );
        }

        const activeClass = this.currentFile === entry.path ? " active" : "";
        return (
          `<div class="tree-item${activeClass}" data-file="${escapeHtml(entry.path)}" data-size="${entry.size}" style="padding-left:${paddingLeft + 16}px">` +
          `<span class="icon">${getFileIcon(entry.name, false)}</span>` +
          `<span>${escapeHtml(entry.name)}</span>` +
          `</div>`
        );
      }).join("");
    },

    _attachTreeListeners(container) {
      container.querySelectorAll("[data-dir]").forEach((el) => {
        el.addEventListener("click", () => {
          const p = el.getAttribute("data-dir");
          this.expandedDirs[p] = !this.expandedDirs[p];
          this._renderTree();
        });
      });

      container.querySelectorAll("[data-file]").forEach((el) => {
        el.addEventListener("click", () => {
          this.loadFile(
            el.getAttribute("data-file"),
            parseInt(el.getAttribute("data-size") || "0")
          );
        });
      });
    },

    // ----------------------------------------------------------------
    // File loading & code display
    // ----------------------------------------------------------------

    async loadFile(filePath, size) {
      this.currentFile = filePath;
      this.fileSize = formatBytes(size);
      this.fileLoading = true;
      this._renderTree(); // refresh active highlight

      try {
        const resp = await apiFetch(
          `/admin/api/plugins/${this.pluginID}/file` +
          `?ref=${encodeURIComponent(this.currentRef)}&path=${encodeURIComponent(filePath)}`
        );
        if (!resp.ok) throw new Error("Failed to load file");
        const data = await resp.json();
        this._displayCode(data.content, filePath);
      } catch (err) {
        showToast("Error loading file: " + err.message, "error");
      } finally {
        this.fileLoading = false;
      }
    },

    _displayCode(content, filePath) {
      const codeEl = document.getElementById("code-display");
      const lineNumEl = document.getElementById("line-numbers");
      if (!codeEl || !lineNumEl) return;

      const lang = getLanguageForFile(filePath);
      let highlighted;
      try {
        highlighted = hljs.getLanguage(lang)
          ? hljs.highlight(content, { language: lang }).value
          : hljs.highlightAuto(content).value;
      } catch {
        highlighted = escapeHtml(content);
      }

      codeEl.innerHTML = highlighted;

      // Generate line numbers
      const lineCount = content.split("\n").length;
      lineNumEl.textContent = Array.from({ length: lineCount }, (_, i) => i + 1).join("\n");
    },
  };
}
