// draftSync centralizes the decision of when the message composer's local
// draft text should be (re)seeded from an *external* source — i.e. the
// conversation row delivered by the server over the conversation-list patch
// stream — versus when that external value must be IGNORED because it is a
// stale echo of the user's own in-flight autosave.
//
// Why this exists (the bug it fixes):
//
//   While composing in a draft conversation, every keystroke is autosaved with
//   a debounced PUT /conversation/<id>/draft. The server bumps updated_at so
//   the conversation reorders, which re-emits the conversation row — INCLUDING
//   its `draft` text — back to every client over the list-patch stream. The
//   composer is a controlled textarea fed from that row, so a naive "copy the
//   server draft into local state whenever it changes" effect would feed the
//   echo back into the textarea.
//
//   Over a high-latency link the echo is STALE: by the time it arrives the user
//   has typed further (or backspaced and retyped), so re-applying the older
//   snapshot deletes the characters typed since — the reported "the box deletes
//   the text I am typing" bug.
//
// The rule: local edits own the draft for the duration of an editing session.
// The external value is adopted exactly once per session, when the composer
// first enters a conversation (a genuine context switch) — never on the
// subsequent echoes for the SAME conversation.
//
// A "session" is identified by the conversation id the composer is bound to.
// `null` is the special "new conversation" session. A lazily-created draft
// (conversationId flips from null to a fresh id mid-typing, without remounting
// the composer) is treated as the SAME session as that `null` it grew out of:
// its very first server row carries the snapshot captured at create time, which
// is already stale relative to the live textarea, so it must never be adopted.

export interface DraftSyncState {
  /** The conversation id the composer is currently bound to (null = new). */
  conversationId: string | null;
  /** Id of a draft lazily created from the current "new conversation" input
   * session, or null. When equal to conversationId, we're still in the
   * session that began as `null` and must not adopt the server snapshot. */
  lazyDraftId: string | null;
  /** Whether the current conversation row is a draft. */
  isDraft: boolean;
  /** The draft text on the current conversation row (server-owned echo). */
  serverDraft: string;
  /** Whether the conversation row for `conversationId` has actually loaded.
   *
   * This guards the real-app timing hole: navigating to /c/<id> sets
   * conversationId synchronously from the URL, but currentConversation is
   * derived from the async-loaded conversation list/stream, so there is at
   * least one render where conversationId is non-null yet the row is absent
   * (isDraft=false, serverDraft=""). Without this flag we would finalize the
   * session as "non-draft, empty" on that first render and then ignore the
   * row's later arrival as a same-session echo -- leaving the textarea blank
   * for an existing draft. While a non-null conversation is still loading we
   * DEFER the decision instead. Always true for the `null` new-conversation
   * view (there is no row to wait for) and for a lazily-created draft (its id
   * is known synchronously). */
  conversationLoaded: boolean;
  /** The session id we last seeded local draft state for, as returned by a
   * previous call (start with `NO_SESSION`). */
  initializedFor: string | null | typeof NO_SESSION;
}

export interface DraftSyncDecision {
  /** Whether to overwrite local draft state with `value`. */
  adopt: boolean;
  /** The value to adopt (only meaningful when adopt === true). */
  value: string;
  /** The session id to remember for the next call. Pass this back in as
   * `initializedFor`. */
  initializedFor: string | null | typeof NO_SESSION;
}

/** Sentinel distinct from every real session id (including the `null` "new"
 * session), so the very first decision is always treated as "entering". */
export const NO_SESSION = Symbol("draftSync.noSession");

// The session a given UI state belongs to. A lazily-created draft shares the
// `null` session it grew out of, so its stale create-time snapshot is ignored.
function sessionOf(state: DraftSyncState): string | null {
  if (state.lazyDraftId !== null && state.conversationId === state.lazyDraftId) {
    return null;
  }
  return state.conversationId;
}

// Decide whether to seed local draft text from the server row. Adopt only on a
// genuine session change; never on an echo within the same session.
export function decideDraftSync(state: DraftSyncState): DraftSyncDecision {
  const session = sessionOf(state);

  // Same session as last time → this is an echo (or an unrelated re-render).
  // Local edits own the draft; do not clobber.
  if (state.initializedFor !== NO_SESSION && state.initializedFor === session) {
    return { adopt: false, value: "", initializedFor: session };
  }

  // Entering a new session. For a real (non-null) conversation we must wait
  // for its row to load before deciding: acting on the not-yet-loaded render
  // would wrongly finalize the session as a non-draft and discard the real
  // row when it arrives. Defer WITHOUT recording initializedFor, so the next
  // render (and ultimately the row's arrival) is still treated as entry.
  // The `null` new-view and lazy drafts are known synchronously, so they skip
  // this wait (conversationLoaded is always true for them).
  if (state.conversationId !== null && !state.conversationLoaded) {
    return { adopt: false, value: "", initializedFor: state.initializedFor };
  }

  if (state.lazyDraftId !== null && state.conversationId === state.lazyDraftId) {
    // The lazy-draft row's snapshot is stale relative to the live textarea;
    // never adopt it. (Same session as `null`, recorded so future echoes for
    // this id are also ignored.)
    return { adopt: false, value: "", initializedFor: session };
  }
  if (state.isDraft) {
    return { adopt: true, value: state.serverDraft, initializedFor: session };
  }
  if (state.conversationId === null) {
    // Fresh new-conversation view: start empty.
    return { adopt: true, value: "", initializedFor: session };
  }
  // A non-draft conversation: the composer is keyed by conversationId and
  // remounts, re-seeding from its own state, so there's nothing to adopt here.
  return { adopt: false, value: "", initializedFor: session };
}
