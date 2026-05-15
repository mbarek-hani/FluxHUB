/**
 * diff-viewer.js — Alpine.js component for comparing plugin versions.
 *
 * Used on: /admin/plugins/:id/diff
 */
import { apiFetch, showToast, escapeHtml } from './utils.js';

/**
 * @param {string}   pluginID
 * @param {string[]} tags    - Available tags for the ref selectors
 * @param {string}   fromRef - Pre-selected base ref (from URL query)
 * @param {string}   toRef   - Pre-selected compare ref (from URL query)
 */
export function diffViewer(pluginID, tags, fromRef, toRef) {
  return {
    pluginID,
    tags: tags || [],
    fromRef: fromRef || "",
    toRef:   toRef   || "",

    loading:    false,
    diffLoaded: false,
    rawDiff:    "",
    diffStats:  { files: 0, additions: 0, deletions: 0 },

    init() {
      if (this.fromRef && this.toRef) this.loadDiff();
    },

    // ----------------------------------------------------------------
    // Diff loading
    // ----------------------------------------------------------------

    async loadDiff() {
      if (!this.fromRef || !this.toRef) return;
      this.loading    = true;
      this.diffLoaded = false;

      try {
        const resp = await apiFetch(
          `/admin/api/plugins/${this.pluginID}/diff` +
          `?from=${encodeURIComponent(this.fromRef)}&to=${encodeURIComponent(this.toRef)}`
        );
        if (!resp.ok) {
          const err = await resp.json();
          throw new Error(err.error || "Failed to load diff");
        }

        const data = await resp.json();
        this.rawDiff = data.diff;
        this._renderDiff(data.diff);
        this.diffLoaded = true;

        // Persist selection in URL without a page reload
        const url = new URL(window.location);
        url.searchParams.set("from", this.fromRef);
        url.searchParams.set("to",   this.toRef);
        history.replaceState(null, "", url);
      } catch (err) {
        showToast("Error: " + err.message, "error");
      } finally {
        this.loading = false;
      }
    },

    // ----------------------------------------------------------------
    // Rendering
    // ----------------------------------------------------------------

    _renderDiff(diffText) {
      const container = document.getElementById("diff-output");
      if (!container) return;

      if (!diffText?.trim()) {
        container.innerHTML =
          '<div class="diff-container"><div class="diff-file-header"><span>No differences found between these references.</span></div></div>';
        this.diffStats = { files: 0, additions: 0, deletions: 0 };
        return;
      }

      const files = this._parseDiff(diffText);
      let totalAdditions = 0;
      let totalDeletions = 0;

      container.innerHTML = files.map((file) => {
        totalAdditions += file.additions;
        totalDeletions += file.deletions;

        const header =
          `<div class="diff-file-header" onclick="this.nextElementSibling.style.display = this.nextElementSibling.style.display === 'none' ? 'table' : 'none'">` +
          `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>` +
          `<span>${escapeHtml(file.filename)}</span>` +
          `<span class="additions">+${file.additions}</span>` +
          `<span class="deletions">-${file.deletions}</span>` +
          this._renderStatsBar(file.additions, file.deletions) +
          `</div>`;

        const rows = file.lines.map((l) => this._renderDiffLine(l)).join("");
        const table =
          `<table class="diff-table">` +
          `<colgroup><col style="width:50px"><col style="width:50px"><col style="width:20px"><col></colgroup>` +
          `<tbody>${rows}</tbody></table>`;

        return `<div class="diff-container">${header}${table}</div>`;
      }).join("");

      // Syntax-highlight each code cell
      container.querySelectorAll(".diff-line-content code").forEach((block) => {
        try { hljs.highlightElement(block); } catch { /* ignore */ }
      });

      this.diffStats = { files: files.length, additions: totalAdditions, deletions: totalDeletions };
    },

    _renderDiffLine(line) {
      if (line.type === "hunk") {
        return `<tr class="diff-hunk"><td colspan="4">${escapeHtml(line.content)}</td></tr>`;
      }

      const rowClass = "diff-" + line.type;
      const marker   = line.type === "add" ? "+" : line.type === "del" ? "-" : " ";

      return (
        `<tr class="${rowClass}">` +
        `<td class="diff-line-num">${line.oldNum}</td>` +
        `<td class="diff-line-num">${line.newNum}</td>` +
        `<td class="diff-line-marker">${marker}</td>` +
        `<td class="diff-line-content"><code>${escapeHtml(line.content || "")}</code></td>` +
        `</tr>`
      );
    },

    _renderStatsBar(additions, deletions) {
      const total = additions + deletions;
      if (total === 0) return "";
      const max       = 5;
      const addBlocks = Math.round((additions / total) * max);
      const delBlocks = max - addBlocks;
      return (
        '<span class="diff-stats-bar">' +
        '<span class="add"></span>'.repeat(addBlocks) +
        '<span class="del"></span>'.repeat(delBlocks) +
        "</span>"
      );
    },

    // ----------------------------------------------------------------
    // Diff parsing
    // ----------------------------------------------------------------

    _parseDiff(text) {
      return text
        .split(/^diff --git /m)
        .filter(Boolean)
        .map((chunk) => this._parseChunk(chunk))
        .filter((f) => f.filename);
    },

    _parseChunk(chunk) {
      const lines    = chunk.split("\n");
      const match    = (lines[0] || "").match(/b\/(.+)$/);
      const filename = match ? match[1] : "";

      let additions   = 0;
      let deletions   = 0;
      let oldLine     = 0;
      let newLine     = 0;
      let inBody      = false;
      const parsedLines = [];

      const SKIP = /^(index |---|[+]{3}|new file|deleted file|similarity|rename)/;

      for (let i = 1; i < lines.length; i++) {
        const line = lines[i];
        if (SKIP.test(line)) continue;

        const hunkMatch = line.match(/^@@ -(\d+),?\d* \+(\d+),?\d* @@(.*)/);
        if (hunkMatch) {
          oldLine = parseInt(hunkMatch[1]);
          newLine = parseInt(hunkMatch[2]);
          inBody  = true;
          parsedLines.push({ type: "hunk", content: line, text: hunkMatch[3] || "" });
          continue;
        }

        if (!inBody) continue;

        if (line.startsWith("+")) {
          additions++;
          parsedLines.push({ type: "add",     oldNum: "",      newNum: newLine++, content: line.slice(1) });
        } else if (line.startsWith("-")) {
          deletions++;
          parsedLines.push({ type: "del",     oldNum: oldLine++, newNum: "",      content: line.slice(1) });
        } else if (line.startsWith(" ")) {
          parsedLines.push({ type: "neutral", oldNum: oldLine++, newNum: newLine++, content: line.slice(1) });
        } else if (line === "\\ No newline at end of file") {
          parsedLines.push({ type: "neutral", oldNum: "",       newNum: "",        content: "⏎ No newline at end of file" });
        }
      }

      return { filename, additions, deletions, lines: parsedLines };
    },
  };
}
