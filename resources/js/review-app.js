/**
 * review-app.js — Alpine.js component for the admin plugin review actions.
 *
 * Used on: /admin/plugins/:id/review
 */
import { apiFetch, showToast } from './utils.js';

/**
 * @param {string} pluginID
 */
export function reviewApp(pluginID) {
  return {
    pluginID,
    approveVersion: "",
    approveComment: "",
    rejectReason:   "",
    loading:        false,

    async approve() {
      if (!this.approveVersion) return;
      this.loading = true;

      try {
        const resp = await apiFetch(`/admin/api/plugins/${this.pluginID}/approve`, {
          method: "POST",
          body: JSON.stringify({
            version: this.approveVersion,
            comment: this.approveComment,
          }),
        });

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
        const resp = await apiFetch(`/admin/api/plugins/${this.pluginID}/reject`, {
          method: "POST",
          body: JSON.stringify({ reason: this.rejectReason }),
        });

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
