// Standalone conversation export view, served at /export/<conversation_id>.
//
// This is a real, bookmarkable route (not a blob: document), so it opens and
// refreshes like any normal page and avoids the browser "download blob:"
// quirks. It fetches the conversation, converts it to Markdown on the client
// (conversationToMarkdown), and shows a split editor: editable Markdown source
// on the left, live-rendered preview on the right. It reuses the app's bundled
// marked + DOMPurify (via MarkdownContent), so it works offline.
import React, { useEffect, useMemo, useRef, useState, useCallback } from "react";
import { api } from "../services/api";
import { Conversation, Message } from "../types";
import { conversationToMarkdown } from "../utils/conversationMarkdown";
import MarkdownContent from "./MarkdownContent";

function filenameFor(conversation: Conversation | null): string {
  const base = (conversation?.slug || "conversation")
    .replace(/[^a-z0-9-_]+/gi, "-")
    .replace(/^-+|-+$/g, "")
    .toLowerCase();
  return `${base || "conversation"}.md`;
}

function download(name: string, text: string, mime: string) {
  const blob = new Blob([text], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
}

async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(text);
  }
  // Fallback for non-secure contexts.
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  document.execCommand("copy");
  document.body.removeChild(ta);
}

// Extract the /export/<id> conversation id from the current path.
export function exportConversationIdFromPath(): string | null {
  const m = window.location.pathname.match(/^\/export\/([^/]+)\/?$/);
  return m ? decodeURIComponent(m[1]) : null;
}

type MobilePane = "edit" | "preview";

function ExportPage({ conversationId }: { conversationId: string }) {
  const [conversation, setConversation] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [includeToolOutputs, setIncludeToolOutputs] = useState(true);
  // The editable source. We seed it from the generated markdown and let the
  // user edit; toggling the checkbox regenerates (with an edit guard).
  const [source, setSource] = useState("");
  const [edited, setEdited] = useState(false);
  const [mobilePane, setMobilePane] = useState<MobilePane>("edit");
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getConversationWithProgress(conversationId)
      .then((resp) => {
        if (cancelled) return;
        setConversation(resp.conversation ?? null);
        setMessages(resp.messages ?? []);
        setLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : String(err));
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [conversationId]);

  // Markdown generated from the current options. Memoized so toggling the
  // checkbox is cheap and the edit-guard can compare against it.
  const generated = useMemo(
    () =>
      conversationToMarkdown(conversation ?? undefined, messages, {
        includeToolOutputs,
      }),
    [conversation, messages, includeToolOutputs],
  );

  // Seed the editor exactly once, when the conversation finishes loading.
  // (Checkbox toggles re-seed explicitly in onToggleToolOutputs; we don't want
  // `generated` changing to clobber the editor on every render.)
  const [seeded, setSeeded] = useState(false);
  useEffect(() => {
    if (!seeded && !loading && !error) {
      setSource(generated);
      setEdited(false);
      setSeeded(true);
    }
  }, [seeded, loading, error, generated]);

  useEffect(() => {
    if (conversation) {
      document.title = `${conversation.slug || "Conversation"} \u2014 Export`;
    }
  }, [conversation]);

  const toastTimer = useRef<number | undefined>(undefined);
  const showToast = useCallback((msg: string) => {
    setToast(msg);
    window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToast(null), 1600);
  }, []);

  const onToggleToolOutputs = (next: boolean) => {
    // Regenerate with the new option. Guard against clobbering hand edits.
    const nextMd = conversationToMarkdown(conversation ?? undefined, messages, {
      includeToolOutputs: next,
    });
    if (edited && source !== generated) {
      if (!window.confirm("Switching tool outputs will discard your edits. Continue?")) {
        return;
      }
    }
    setIncludeToolOutputs(next);
    setSource(nextMd);
    setEdited(false);
  };

  const filename = filenameFor(conversation);

  if (loading) {
    return (
      <div className="export-page export-centered">
        <div className="export-spinner" aria-label="Loading" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="export-page export-centered">
        <div className="export-error">Failed to load conversation: {error}</div>
      </div>
    );
  }

  return (
    <div className="export-page">
      <header className="export-bar">
        <div className="export-title" title={conversation?.slug || "Conversation"}>
          {conversation?.slug || "Conversation"}
        </div>
        <label className="export-opt">
          <input
            type="checkbox"
            checked={includeToolOutputs}
            onChange={(e) => onToggleToolOutputs(e.target.checked)}
          />
          Include tool outputs
        </label>
        <div className="export-tabs" role="tablist">
          <button
            className={`export-tab${mobilePane === "edit" ? " export-tab-active" : ""}`}
            onClick={() => setMobilePane("edit")}
            role="tab"
          >
            Markdown
          </button>
          <button
            className={`export-tab${mobilePane === "preview" ? " export-tab-active" : ""}`}
            onClick={() => setMobilePane("preview")}
            role="tab"
          >
            Preview
          </button>
        </div>
      </header>

      <main className="export-panes">
        <section
          className={`export-pane export-pane-edit${mobilePane === "edit" ? " export-pane-shown" : ""}`}
          aria-label="Markdown source"
        >
          <div className="export-pane-head">
            <span className="export-pane-label">Markdown</span>
            <div className="export-pane-actions">
              <button
                className="export-btn"
                onClick={() =>
                  copyText(source).then(
                    () => showToast("Markdown copied"),
                    () => showToast("Copy failed"),
                  )
                }
              >
                Copy
              </button>
              <button
                className="export-btn export-btn-primary"
                onClick={() => {
                  download(filename, source, "text/markdown");
                  showToast(`Downloaded ${filename}`);
                }}
              >
                Download .md
              </button>
            </div>
          </div>
          <textarea
            className="export-src"
            spellCheck={false}
            value={source}
            onChange={(e) => {
              setSource(e.target.value);
              setEdited(true);
            }}
            aria-label="Editable markdown"
          />
        </section>

        <section
          className={`export-pane export-pane-preview${mobilePane === "preview" ? " export-pane-shown" : ""}`}
          aria-label="Rendered preview"
        >
          <div className="export-pane-head">
            <span className="export-pane-label">Preview</span>
            <div className="export-pane-actions">
              <button
                className="export-btn"
                onClick={() =>
                  copyText(source).then(
                    () => showToast("Copied as text"),
                    () => showToast("Copy failed"),
                  )
                }
              >
                Copy
              </button>
            </div>
          </div>
          <article className="export-preview markdown-content">
            <MarkdownContent text={source} />
          </article>
        </section>
      </main>

      {toast && <div className="export-toast export-toast-show">{toast}</div>}
    </div>
  );
}

export default ExportPage;
