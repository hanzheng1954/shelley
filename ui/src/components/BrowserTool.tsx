import React from "react";
import { LLMContent } from "../types";
import BrowserNavigateTool from "./BrowserNavigateTool";
import BrowserEvalTool from "./BrowserEvalTool";
import BrowserResizeTool from "./BrowserResizeTool";
import BrowserConsoleLogsTool from "./BrowserConsoleLogsTool";
import BrowserScreencastTool from "./BrowserScreencastTool";
import BrowserEmulateTool from "./BrowserEmulateTool";
import BrowserNetworkTool from "./BrowserNetworkTool";
import BrowserAccessibilityTool from "./BrowserAccessibilityTool";
import BrowserProfileTool from "./BrowserProfileTool";
import ScreenshotTool from "./ScreenshotTool";
import GenericTool from "./GenericTool";

interface BrowserToolProps {
  toolInput?: unknown;
  isRunning?: boolean;
  toolResult?: LLMContent[];
  hasError?: boolean;
  executionTime?: string;
  display?: unknown;
}

function getAction(toolInput: unknown): string {
  if (
    typeof toolInput === "object" &&
    toolInput !== null &&
    "action" in toolInput &&
    typeof (toolInput as Record<string, unknown>).action === "string"
  ) {
    return (toolInput as Record<string, unknown>).action as string;
  }
  return "";
}

// The emulate/network/accessibility/profile families are folded into the
// single "browser" tool as `<family>_<sub>` actions (e.g. "emulate_device").
// The specialized sub-components were written for the old standalone tools,
// where `action` was just the sub-action ("device"). Rewrite the input's
// `action` to that bare sub-action so those components render clean summaries
// for both old data (separate tools) and new combined data.
function withSubAction(toolInput: unknown, family: string): unknown {
  const action = getAction(toolInput);
  const prefix = `${family}_`;
  if (!action.startsWith(prefix)) return toolInput;
  const base = typeof toolInput === "object" && toolInput !== null ? toolInput : {};
  return { ...(base as Record<string, unknown>), action: action.slice(prefix.length) };
}

function BrowserTool(props: BrowserToolProps) {
  const action = getAction(props.toolInput);
  const family = action.split("_", 1)[0];

  switch (action) {
    case "navigate":
      return <BrowserNavigateTool {...props} />;
    case "eval":
      return <BrowserEvalTool {...props} />;
    case "resize":
      return <BrowserResizeTool {...props} />;
    case "screenshot":
      return <ScreenshotTool {...props} />;
    case "console_logs":
      return <BrowserConsoleLogsTool toolName="browser_recent_console_logs" {...props} />;
    case "clear_console_logs":
      return <BrowserConsoleLogsTool toolName="browser_clear_console_logs" {...props} />;
    case "screencast_start":
    case "screencast_stop":
    case "screencast_status":
      return <BrowserScreencastTool {...props} />;
    default:
      break;
  }

  switch (family) {
    case "emulate":
      return (
        <BrowserEmulateTool {...props} toolInput={withSubAction(props.toolInput, "emulate")} />
      );
    case "network":
      return (
        <BrowserNetworkTool {...props} toolInput={withSubAction(props.toolInput, "network")} />
      );
    case "accessibility":
      return (
        <BrowserAccessibilityTool
          {...props}
          toolInput={withSubAction(props.toolInput, "accessibility")}
        />
      );
    case "profile":
      return (
        <BrowserProfileTool {...props} toolInput={withSubAction(props.toolInput, "profile")} />
      );
    default:
      return <GenericTool toolName={`browser (${action || "unknown"})`} {...props} />;
  }
}

export default BrowserTool;
