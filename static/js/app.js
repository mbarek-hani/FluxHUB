// ============================================
// FLUX MARKETPLACE — Admin UI JavaScript
// Alpine.js components + Utilities
// ============================================

// ---- Utility Functions ----

function showToast(message, type) {
  const container = document.getElementById("toasts");
  const toast = document.createElement("div");
  toast.className = "toast toast-" + type;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => toast.remove(), 4000);
}

async function apiFetch(url, options) {
  const defaults = {
    headers: { "Content-Type": "application/json" },
    credentials: "same-origin",
  };
  const merged = { ...defaults, ...options };
  if (options && options.headers) {
    merged.headers = { ...defaults.headers, ...options.headers };
  }
  const resp = await fetch(url, merged);
  return resp;
}

function getLanguageForFile(filename) {
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
    "blade.php": "php",
    yml: "yaml",
    yaml: "yaml",
    env: "bash",
    txt: "plaintext",
  };
  // Check for blade.php
  if (filename.endsWith(".blade.php")) return "php";
  return map[ext] || "plaintext";
}

function getFileIcon(name, isDir, isOpen) {
  if (isDir) {
    if (isOpen) {
      return '<svg class="icon-folder-open" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
    }
    return '<svg class="icon-folder" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M10 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-8l-2-2z"/></svg>';
  }

  const ext = name.split(".").pop().toLowerCase();
  let colorClass = "";
  if (name.endsWith(".blade.php")) colorClass = "icon-blade";
  else if (ext === "php") colorClass = "icon-php";
  else if (ext === "json") colorClass = "icon-json";
  else if (ext === "md") colorClass = "icon-md";
  else if (ext === "js") colorClass = "icon-js";
  else if (ext === "css") colorClass = "icon-css";

  return (
    '<svg class="' +
    colorClass +
    '" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>'
  );
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

// ============================================
// FILE BROWSER - Alpine.js Component
// ============================================

function fileBrowser(pluginID, initialRef, tags) {
  return {
    pluginID: pluginID,
    currentRef: initialRef || "HEAD",
    tags: tags || [],
    tree: [],
    flatTree: [],
    currentFile: "",
    fileSize: "",
    fileContent: "",
    loading: false,
    fileLoading: false,
    fileCount: 0,
    expandedDirs: {},

    init() {
      this.loadTree();
    },

    async loadTree() {
      this.loading = true;
      this.currentFile = "";
      this.fileContent = "";
      try {
        const resp = await apiFetch(
          "/admin/api/plugins/" +
            this.pluginID +
            "/tree?ref=" +
            encodeURIComponent(this.currentRef),
        );
        if (!resp.ok) throw new Error("Failed to load tree");
        this.tree = await resp.json();
        this.flatTree = this.flattenTree(this.tree);
        this.fileCount = this.countFiles(this.tree);
        this.expandedDirs = {};
        // Expand first level
        this.tree.forEach((entry) => {
          if (entry.is_dir) this.expandedDirs[entry.path] = true;
        });
        this.renderTree();
      } catch (err) {
        showToast("Error loading file tree: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },

    flattenTree(entries) {
      let result = [];
      for (const entry of entries) {
        result.push(entry);
        if (entry.is_dir && entry.children) {
          result = result.concat(this.flattenTree(entry.children));
        }
      }
      return result;
    },

    countFiles(entries) {
      let count = 0;
      for (const entry of entries) {
        if (!entry.is_dir) count++;
        if (entry.children) count += this.countFiles(entry.children);
      }
      return count;
    },

    renderTree() {
      const container = document.getElementById("file-tree-root");
      if (!container) return;
      container.innerHTML = this.buildTreeHTML(this.tree, 0);
      this.attachTreeListeners(container);
    },

    buildTreeHTML(entries, depth) {
      let html = "";
      // Sort: dirs first, then alphabetical
      const sorted = [...entries].sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        return a.name.localeCompare(b.name);
      });

      for (const entry of sorted) {
        const padding = 12 + depth * 16;
        const isExpanded = this.expandedDirs[entry.path];

        if (entry.is_dir) {
          const chevron = isExpanded
            ? '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>'
            : '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>';

          html +=
            '<div class="tree-item" data-dir="' +
            escapeHtml(entry.path) +
            '" style="padding-left:' +
            padding +
            'px">';
          html +=
            '<span class="icon" style="width:12px">' + chevron + "</span>";
          html +=
            '<span class="icon">' +
            getFileIcon(entry.name, true, isExpanded) +
            "</span>";
          html += "<span>" + escapeHtml(entry.name) + "</span>";
          html += "</div>";

          if (isExpanded && entry.children && entry.children.length > 0) {
            html += '<div class="tree-children">';
            html += this.buildTreeHTML(entry.children, depth + 1);
            html += "</div>";
          }
        } else {
          const activeClass = this.currentFile === entry.path ? " active" : "";
          html +=
            '<div class="tree-item' +
            activeClass +
            '" data-file="' +
            escapeHtml(entry.path) +
            '" data-size="' +
            entry.size +
            '" style="padding-left:' +
            (padding + 16) +
            'px">';
          html +=
            '<span class="icon">' + getFileIcon(entry.name, false) + "</span>";
          html += "<span>" + escapeHtml(entry.name) + "</span>";
          html += "</div>";
        }
      }
      return html;
    },

    attachTreeListeners(container) {
      const self = this;

      container.querySelectorAll("[data-dir]").forEach((el) => {
        el.addEventListener("click", function () {
          const dirPath = this.getAttribute("data-dir");
          self.expandedDirs[dirPath] = !self.expandedDirs[dirPath];
          self.renderTree();
        });
      });

      container.querySelectorAll("[data-file]").forEach((el) => {
        el.addEventListener("click", function () {
          const filePath = this.getAttribute("data-file");
          const size = parseInt(this.getAttribute("data-size") || "0");
          self.loadFile(filePath, size);
        });
      });
    },

    async loadFile(filePath, size) {
      this.currentFile = filePath;
      this.fileSize = formatBytes(size);
      this.fileLoading = true;
      this.renderTree(); // Update active state

      try {
        const resp = await apiFetch(
          "/admin/api/plugins/" +
            this.pluginID +
            "/file?ref=" +
            encodeURIComponent(this.currentRef) +
            "&path=" +
            encodeURIComponent(filePath),
        );
        if (!resp.ok) throw new Error("Failed to load file");
        const data = await resp.json();

        this.displayCode(data.content, filePath);
      } catch (err) {
        showToast("Error loading file: " + err.message, "error");
      } finally {
        this.fileLoading = false;
      }
    },

    displayCode(content, filePath) {
      const codeEl = document.getElementById("code-display");
      const lineNumEl = document.getElementById("line-numbers");
      if (!codeEl || !lineNumEl) return;

      const lang = getLanguageForFile(filePath);
      let highlighted;

      try {
        if (hljs.getLanguage(lang)) {
          highlighted = hljs.highlight(content, { language: lang }).value;
        } else {
          highlighted = hljs.highlightAuto(content).value;
        }
      } catch (e) {
        highlighted = escapeHtml(content);
      }

      codeEl.innerHTML = highlighted;

      // Line numbers
      const lines = content.split("\n");
      let nums = "";
      for (let i = 1; i <= lines.length; i++) {
        nums += i + "\n";
      }
      lineNumEl.textContent = nums;
    },
  };
}

// ============================================
// DIFF VIEWER - Alpine.js Component
// ============================================

function diffViewer(pluginID, tags, fromRef, toRef) {
  return {
    pluginID: pluginID,
    tags: tags || [],
    fromRef: fromRef || "",
    toRef: toRef || "",
    loading: false,
    diffLoaded: false,
    rawDiff: "",
    diffStats: { files: 0, additions: 0, deletions: 0 },

    init() {
      if (this.fromRef && this.toRef) {
        this.loadDiff();
      }
    },

    async loadDiff() {
      if (!this.fromRef || !this.toRef) return;
      this.loading = true;
      this.diffLoaded = false;

      try {
        const resp = await apiFetch(
          "/admin/api/plugins/" +
            this.pluginID +
            "/diff?from=" +
            encodeURIComponent(this.fromRef) +
            "&to=" +
            encodeURIComponent(this.toRef),
        );
        if (!resp.ok) {
          const err = await resp.json();
          throw new Error(err.error || "Failed to load diff");
        }
        const data = await resp.json();
        this.rawDiff = data.diff;
        this.renderDiff(data.diff);
        this.diffLoaded = true;

        // Update URL
        const url = new URL(window.location);
        url.searchParams.set("from", this.fromRef);
        url.searchParams.set("to", this.toRef);
        history.replaceState(null, "", url);
      } catch (err) {
        showToast("Error: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },

    renderDiff(diffText) {
      const container = document.getElementById("diff-output");
      if (!container) return;

      if (!diffText || diffText.trim() === "") {
        container.innerHTML =
          '<div class="panel"><div class="empty-state"><p>No differences found between these references.</p></div></div>';
        this.diffStats = { files: 0, additions: 0, deletions: 0 };
        return;
      }

      const files = this.parseDiff(diffText);
      let totalAdditions = 0;
      let totalDeletions = 0;

      let html = "";

      for (const file of files) {
        totalAdditions += file.additions;
        totalDeletions += file.deletions;

        html += '<div class="diff-container" style="margin-bottom:16px">';

        // File header
        html +=
          "<div class=\"diff-file-header\" onclick=\"this.nextElementSibling.style.display = this.nextElementSibling.style.display === 'none' ? 'table' : 'none'\">";
        html +=
          '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>';
        html += "<span>" + escapeHtml(file.filename) + "</span>";
        html += '<span class="additions">+' + file.additions + "</span>";
        html += '<span class="deletions">-' + file.deletions + "</span>";
        html += this.renderStatsBar(file.additions, file.deletions);
        html += "</div>";

        // Diff table
        html += '<table class="diff-table">';
        html += "<colgroup>";
        html += '<col style="width:50px">';
        html += '<col style="width:50px">';
        html += '<col style="width:20px">';
        html += "<col>";
        html += "</colgroup>";
        html += "<tbody>";

        for (const line of file.lines) {
          html += this.renderDiffLine(line);
        }

        html += "</tbody></table>";
        html += "</div>";
      }

      container.innerHTML = html;

      // Apply syntax highlighting to diff code
      container.querySelectorAll(".diff-line-content code").forEach((block) => {
        try {
          hljs.highlightElement(block);
        } catch (e) {
          /* ignore */
        }
      });

      this.diffStats = {
        files: files.length,
        additions: totalAdditions,
        deletions: totalDeletions,
      };
    },

    parseDiff(text) {
      const files = [];
      const fileChunks = text.split(/^diff --git /m).filter(Boolean);

      for (const chunk of fileChunks) {
        const lines = chunk.split("\n");
        let filename = "";

        // Extract filename from the first line
        const firstLine = lines[0] || "";
        const match = firstLine.match(/b\/(.+)$/);
        if (match) {
          filename = match[1];
        }

        let additions = 0;
        let deletions = 0;
        const parsedLines = [];

        let oldLine = 0;
        let newLine = 0;
        let inBody = false;

        for (let i = 1; i < lines.length; i++) {
          const line = lines[i];

          // Skip file metadata lines
          if (
            line.startsWith("index ") ||
            line.startsWith("---") ||
            line.startsWith("+++") ||
            line.startsWith("new file") ||
            line.startsWith("deleted file") ||
            line.startsWith("similarity") ||
            line.startsWith("rename")
          ) {
            continue;
          }

          // Hunk header
          const hunkMatch = line.match(/^@@ -(\d+),?\d* \+(\d+),?\d* @@(.*)/);
          if (hunkMatch) {
            oldLine = parseInt(hunkMatch[1]);
            newLine = parseInt(hunkMatch[2]);
            inBody = true;
            parsedLines.push({
              type: "hunk",
              content: line,
              text: hunkMatch[3] || "",
            });
            continue;
          }

          if (!inBody) continue;

          if (line.startsWith("+")) {
            additions++;
            parsedLines.push({
              type: "add",
              oldNum: "",
              newNum: newLine++,
              content: line.substring(1),
            });
          } else if (line.startsWith("-")) {
            deletions++;
            parsedLines.push({
              type: "del",
              oldNum: oldLine++,
              newNum: "",
              content: line.substring(1),
            });
          } else if (line.startsWith(" ")) {
            parsedLines.push({
              type: "neutral",
              oldNum: oldLine++,
              newNum: newLine++,
              content: line.substring(1),
            });
          } else if (line === "\\ No newline at end of file") {
            parsedLines.push({
              type: "neutral",
              oldNum: "",
              newNum: "",
              content: "⏎ No newline at end of file",
            });
          }
        }

        if (filename) {
          files.push({ filename, additions, deletions, lines: parsedLines });
        }
      }

      return files;
    },

    renderDiffLine(line) {
      if (line.type === "hunk") {
        return (
          '<tr class="diff-hunk"><td colspan="4">' +
          escapeHtml(line.content) +
          "</td></tr>"
        );
      }

      const rowClass = "diff-" + line.type;
      const marker =
        line.type === "add" ? "+" : line.type === "del" ? "-" : " ";

      let contentHtml = escapeHtml(line.content || "");

      return (
        '<tr class="' +
        rowClass +
        '">' +
        '<td class="diff-line-num">' +
        line.oldNum +
        "</td>" +
        '<td class="diff-line-num">' +
        line.newNum +
        "</td>" +
        '<td class="diff-line-marker">' +
        marker +
        "</td>" +
        '<td class="diff-line-content"><code>' +
        contentHtml +
        "</code></td>" +
        "</tr>"
      );
    },

    renderStatsBar(additions, deletions) {
      const total = additions + deletions;
      if (total === 0) return "";

      const max = 5;
      const addBlocks = Math.round((additions / total) * max);
      const delBlocks = max - addBlocks;

      let html = '<span class="diff-stats-bar">';
      for (let i = 0; i < addBlocks; i++) html += '<span class="add"></span>';
      for (let i = 0; i < delBlocks; i++) html += '<span class="del"></span>';
      html += "</span>";
      return html;
    },
  };
}

// ============================================
// REVIEW APP - Alpine.js Component
// ============================================

function reviewApp(pluginID) {
  return {
    pluginID: pluginID,
    approveVersion: "",
    approveComment: "",
    rejectReason: "",
    loading: false,

    async approve() {
      if (!this.approveVersion) return;
      this.loading = true;

      try {
        const resp = await apiFetch(
          "/admin/api/plugins/" + this.pluginID + "/approve",
          {
            method: "POST",
            body: JSON.stringify({
              version: this.approveVersion,
              comment: this.approveComment,
            }),
          },
        );

        const data = await resp.json();

        if (resp.ok) {
          showToast("Plugin approved and signed successfully!", "success");
          setTimeout(() => window.location.reload(), 1500);
        } else {
          showToast("Error: " + (data.error || "Unknown error"), "error");
        }
      } catch (err) {
        showToast("Network error: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },

    async reject() {
      if (!this.rejectReason) return;
      this.loading = true;

      try {
        const resp = await apiFetch(
          "/admin/api/plugins/" + this.pluginID + "/reject",
          {
            method: "POST",
            body: JSON.stringify({
              reason: this.rejectReason,
            }),
          },
        );

        const data = await resp.json();

        if (resp.ok) {
          showToast("Plugin rejected.", "success");
          setTimeout(() => window.location.reload(), 1500);
        } else {
          showToast("Error: " + (data.error || "Unknown error"), "error");
        }
      } catch (err) {
        showToast("Network error: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },
  };
}
