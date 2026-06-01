import { createContext, useContext, useState, type Dispatch, type SetStateAction } from "react";

/** When a tool call is shown in its own detail modal (opened from a tool
 *  pill), the tool's command/headline is already the modal title, so the
 *  card's collapsible body should start *expanded* — there's no point
 *  making the user click a second disclosure to see the detail they just
 *  asked for. In the inline conversation flow this is false, so cards stay
 *  compact. Tool components read this via `useToolExpandedState()`. */
export const ToolDetailContext = createContext<{ defaultExpanded: boolean }>({
  defaultExpanded: false,
});

/** True when this tool card is being shown in its own detail modal
 *  (opened from a tool pill), as opposed to inline in the conversation. */
export function useInToolDetail(): boolean {
  return useContext(ToolDetailContext).defaultExpanded;
}

/** useState for a tool card's primary expand/collapse, seeded from
 *  ToolDetailContext so the card opens expanded inside the detail modal. */
export function useToolExpandedState(): [boolean, Dispatch<SetStateAction<boolean>>] {
  const { defaultExpanded } = useContext(ToolDetailContext);
  return useState(defaultExpanded);
}
