import {
  useEffect,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { useNavigate } from "react-router-dom";
import {
  Link,
  Pencil,
  Play,
  History,
  PlusSquare,
  RefreshCw,
  Upload,
  Wrench,
} from "lucide-react";
import {
  Button,
  FormItem,
  Input,
  Modal,
  Select,
  Textarea,
  toast,
} from "../../../shared/components";
import { AutomationScriptHistoryModal } from "../components/AutomationScriptHistoryModal";
import { AutomationScriptPublicApiModal } from "../components/AutomationScriptPublicApiModal";
import { AutomationScriptRunModal } from "../components/AutomationScriptRunModal";
import { AutomationToolboxModal } from "../components/AutomationToolboxModal";
import { fetchBrowserProfiles } from "../api";
import {
  fetchAutomationScripts,
  importAutomationScriptFromGit,
  importAutomationScriptFromLocalDirectory,
  importAutomationScriptFromLocalFile,
  importAutomationScriptFromLocalLibrary,
  importAutomationScriptFromRemote,
  importAutomationScriptFromText,
  saveAutomationScript,
} from "../automationScriptApi";
import {
  AUTOMATION_SCRIPT_TYPE_OPTIONS,
  createAutomationScriptDraft,
  findAutomationTargetProfile,
  prepareAutomationScriptPublicAPIConfigForSave,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
  type AutomationScriptType,
} from "../automationScripts";
import { useLaunchContext } from "../hooks/useLaunchContext";
import type { BrowserProfile } from "../types";

type ImportMode =
  | "text"
  | "local-file"
  | "local-dir"
  | "local-library"
  | "remote-url"
  | "git";
const DUAL_INSTANCE_SCRIPT_ID = "dual-instance-runtime-switch";
const NEWS_SCRIPT_ID = "news-query-txt";

type DualLaunchCodes = {
  primaryCode: string;
  secondaryCode: string;
};

type AutomationCardPresentation = {
  key: string;
  title: string;
  scriptId?: string;
  scriptType: AutomationScriptType;
  modeLabel: string;
  description: string;
  codeDisplay: string;
  primaryActionLabel: string;
  primaryActionText: string;
  primaryActionSuccessMessage: string;
  secondaryActionLabel: string;
  secondaryActionText: string;
  secondaryActionSuccessMessage: string;
  modeToneClass: string;
  publicAPIEnabled: boolean;
  railClassName: string;
};

function getAutomationCardRailClass(seed: string): string {
  const palette = [
    "bg-[#8aa0b3]",
    "bg-[#8da79b]",
    "bg-[#929ab1]",
    "bg-[#aa9a8e]",
    "bg-[#8b9a9c]",
  ];

  let hash = 0;
  for (const char of seed) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }

  return palette[hash % palette.length];
}

function ScriptCardField({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-9 items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2 shadow-[var(--shadow-sm)]">
      <div className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-text-muted)]">
        {label}
      </div>
      <div className="min-w-0 flex-1 text-[12px] font-medium leading-4 text-[var(--color-text-primary)]">
        {children}
      </div>
    </div>
  );
}

function AutomationScriptSummaryCard({
  card,
  onOpen,
  onRunScript,
  onRunAPI,
}: {
  card: AutomationCardPresentation;
  onOpen?: () => void;
  onRunScript?: () => void;
  onRunAPI?: () => void;
}) {
  const interactive = typeof onOpen === "function";
  const isInterfaceModeCard = card.scriptType === "launch-api";
  const actionButtonClassName =
    "!h-7 !w-[104px] shrink-0 justify-center whitespace-nowrap !rounded-md !border !border-black !bg-black !px-2.5 !text-xs !font-medium !leading-none !text-white !shadow-none hover:!border-[#1f1f1f] hover:!bg-[#1f1f1f] focus-visible:!ring-black disabled:!border-[#6b7280] disabled:!bg-[#6b7280] disabled:!text-white";
  const headerCopyButtonClassName =
    "!h-7 !w-[104px] shrink-0 justify-center whitespace-nowrap !rounded-md !border !border-black !bg-white !px-2.5 !text-xs !font-medium !leading-none !text-black !shadow-none hover:!border-black hover:!bg-[#f3f4f6] hover:!text-black focus-visible:!ring-black disabled:!border-[#6b7280] disabled:!bg-white disabled:!text-[#6b7280]";
  const scriptButtonClassName =
    actionButtonClassName;
  const apiSetupButtonClassName =
    actionButtonClassName;
  const interfaceExecuteButtonClassName =
    actionButtonClassName;
  const editButtonClassName =
    actionButtonClassName;

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!interactive || !onOpen) {
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen();
    }
  };

  return (
    <div
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      onClick={interactive ? onOpen : undefined}
      onKeyDown={interactive ? handleKeyDown : undefined}
      className={`group relative flex h-full flex-col rounded-[22px] border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] pb-3 pl-7 pr-3.5 pt-3 text-left shadow-[var(--shadow-xs)] transition-all duration-200 ${
        interactive
          ? "cursor-pointer hover:border-[var(--color-border-strong)] hover:shadow-[var(--shadow-md)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          : ""
      }`}
    >
      <div
        aria-hidden="true"
        className={`absolute bottom-3 left-3 top-3 w-1 rounded-full ${card.railClassName}`}
      />

      <div className="min-w-0 text-[16px] font-semibold leading-5 text-[var(--color-text-primary)]">
        {card.title}
      </div>

      <div className="mt-2 flex flex-nowrap justify-start gap-1.5 overflow-x-auto">
        <Button
          type="button"
          size="sm"
          className={headerCopyButtonClassName}
          style={{ border: "1px solid #000000", backgroundColor: "#ffffff", color: "#000000" }}
          onClick={(event) => {
            event.stopPropagation();
            void copyToClipboard(
              card.primaryActionText,
              card.primaryActionSuccessMessage,
            );
          }}
        >
          {card.primaryActionLabel}
        </Button>
        <Button
          type="button"
          size="sm"
          className={headerCopyButtonClassName}
          style={{ border: "1px solid #000000", backgroundColor: "#ffffff", color: "#000000" }}
          onClick={(event) => {
            event.stopPropagation();
            void copyToClipboard(
              card.secondaryActionText,
              card.secondaryActionSuccessMessage,
            );
          }}
        >
          {card.secondaryActionLabel}
        </Button>
        {isInterfaceModeCard ? (
          typeof onRunAPI === "function" || typeof onRunScript === "function" ? (
            <Button
              type="button"
              size="sm"
              className={interfaceExecuteButtonClassName}
              onClick={(event) => {
                event.stopPropagation();
                if (typeof onRunAPI === "function") {
                  onRunAPI();
                  return;
                }
                onRunScript?.();
              }}
              aria-label={`执行 ${card.title}`}
              title="执行"
            >
              <Play className="h-3.5 w-3.5" />
              执行
            </Button>
          ) : null
        ) : (
          <>
            {typeof onRunScript === "function" ? (
              <Button
                type="button"
                size="sm"
                className={scriptButtonClassName}
                onClick={(event) => {
                  event.stopPropagation();
                  onRunScript();
                }}
                aria-label={`执行脚本 ${card.title}`}
                title="执行脚本"
              >
                <Play className="h-3.5 w-3.5" />
                脚本
              </Button>
            ) : null}
            {typeof onRunAPI === "function" ? (
              <Button
                type="button"
                size="sm"
                variant="secondary"
                className={
                  card.publicAPIEnabled
                    ? scriptButtonClassName
                    : apiSetupButtonClassName
                }
                onClick={(event) => {
                  event.stopPropagation();
                  onRunAPI();
                }}
                aria-label={`${card.publicAPIEnabled ? "执行接口" : "配置接口"} ${card.title}`}
                title={card.publicAPIEnabled ? "执行接口" : "配置接口"}
              >
                {card.publicAPIEnabled ? (
                  <Play className="h-3.5 w-3.5" />
                ) : (
                  <Link className="h-3.5 w-3.5" />
                )}
                {card.publicAPIEnabled ? "接口" : "配置"}
              </Button>
            ) : null}
          </>
        )}
        {interactive ? (
          <Button
            type="button"
            size="sm"
            className={editButtonClassName}
            onClick={(event) => {
              event.stopPropagation();
              onOpen?.();
            }}
            aria-label={`编辑 ${card.title}`}
            title="编辑"
          >
            <Pencil className="h-3.5 w-3.5" />
            编辑
          </Button>
        ) : null}
      </div>

      <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-[136px_minmax(0,1fr)]">
        <ScriptCardField label="类型">
          <span className="inline-flex items-center gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full ${card.modeToneClass}`} />
            <span>{card.modeLabel}</span>
          </span>
        </ScriptCardField>
        <ScriptCardField label="Code 码">
          <code className="block truncate whitespace-nowrap font-mono text-[10.5px] leading-4 tracking-[0.04em] text-[var(--color-text-primary)]">
            {card.codeDisplay}
          </code>
        </ScriptCardField>
      </div>
    </div>
  );
}

function normalizeText(value?: string): string {
  return String(value || "").trim();
}

function normalizeCode(value?: string): string {
  return normalizeText(value).toUpperCase();
}

function resolveTargetCode(
  selector: AutomationScriptRecord["targetConfig"]["selector"],
  profiles: BrowserProfile[],
): string {
  const matched = findAutomationTargetProfile(selector, profiles);
  return normalizeCode(matched?.launchCode || selector.code);
}

async function copyToClipboard(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}

function parseJSONObjectText(text?: string): Record<string, unknown> | null {
  const normalized = normalizeText(text);
  if (!normalized) {
    return null;
  }

  try {
    const parsed = JSON.parse(normalized);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    return null;
  }

  return null;
}

function buildSelectorPayload(
  selector: AutomationScriptRecord["targetConfig"]["selector"],
  profiles: BrowserProfile[],
): Record<string, unknown> | null {
  const matched = findAutomationTargetProfile(selector, profiles);
  const payload: Record<string, unknown> = {};

  const code = normalizeCode(matched?.launchCode || selector.code);
  const profileId = normalizeText(matched?.profileId || selector.profileId);
  const profileName = normalizeText(
    matched?.profileName || selector.profileName,
  );
  const groupId = normalizeText(selector.groupId);

  if (code) {
    payload.code = code;
  }
  if (profileId) {
    payload.profileId = profileId;
  }
  if (profileName) {
    payload.profileName = profileName;
  }
  if (groupId) {
    payload.groupId = groupId;
  }
  if (selector.keywords.length > 0) {
    payload.keywords = [...selector.keywords];
  }
  if (selector.tags.length > 0) {
    payload.tags = [...selector.tags];
  }

  return Object.keys(payload).length > 0 ? payload : null;
}

function buildAutomationRequestPayload(
  script: AutomationScriptRecord,
  profiles: BrowserProfile[],
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    scriptId: script.id,
  };
  const params = parseJSONObjectText(script.paramsText);

  switch (script.targetConfig.mode) {
    case "existing":
    case "rotate": {
      const selector = buildSelectorPayload(script.targetConfig.selector, profiles);
      if (selector) {
        payload.selector = selector;
      }
      break;
    }
    case "create":
      payload.useScriptSelector = true;
      break;
    default: {
      const selector = parseJSONObjectText(script.selectorText);
      if (selector && Object.keys(selector).length > 0) {
        payload.selector = selector;
      } else if (script.type === "playwright-cdp") {
        payload.selector = { code: "YOUR_CODE" };
      }
      break;
    }
  }

  if (params && Object.keys(params).length > 0) {
    payload.params = params;
  } else {
    payload.useScriptParams = true;
  }

  return payload;
}

function buildAutomationRequestPayloadText(
  payload: Record<string, unknown>,
): string {
  return JSON.stringify(payload, null, 2);
}

function buildAutomationRunCurlDemo(options: {
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  payload: Record<string, unknown>;
}): string {
  const authHeader = buildCurlAuthHeaderLine(
    options.apiAuthEnabled,
    options.apiAuthHeader,
  );
  return `curl -X POST ${options.launchBaseUrl}/api/automation/scripts/run \\
  -H "Content-Type: application/json" \\
${authHeader}  -d '${buildAutomationRequestPayloadText(options.payload)}'`;
}

function buildAutomationCardMode(
  script: AutomationScriptRecord,
): "skill" | "api-sim" {
  return script.type === "playwright-cdp" ? "skill" : "api-sim";
}

function getAutomationModeLabel(type: AutomationScriptType): string {
  return type === "playwright-cdp" ? "脚本模式" : "接口模式";
}

function getAutomationModeToneClass(type: AutomationScriptType): string {
  return type === "playwright-cdp"
    ? "bg-[var(--color-text-primary)]"
    : "bg-[var(--color-text-secondary)]";
}

function buildAutomationSkillPrompt(
  script: AutomationScriptRecord,
  payload: Record<string, unknown>,
) {
  const lines = [
    "使用 ant-chrome-openclaw skill。",
    `执行预置脚本 ${script.id}（${script.name}）。`,
  ];

  if (Object.prototype.hasOwnProperty.call(payload, "selector")) {
    lines.push(`selector: ${JSON.stringify(payload.selector)}`);
  } else if (payload.useScriptSelector) {
    lines.push("selector: 使用脚本默认值。");
  }

  if (Object.prototype.hasOwnProperty.call(payload, "params")) {
    lines.push(`params: ${JSON.stringify(payload.params)}`);
  } else if (payload.useScriptParams) {
    lines.push("params: 使用脚本默认值。");
  }

  return lines.join("\n");
}

function buildAutomationShortDescription(
  script: AutomationScriptRecord,
): string {
  switch (script.id) {
    case DUAL_INSTANCE_SCRIPT_ID:
      return "启动双实例并切换 Runtime";
    case NEWS_SCRIPT_ID:
      return "搜索新闻并写入 TXT";
    default:
      break;
  }

  const source = normalizeText(script.description || script.name);
  const firstSentence = source.split(/[。！？\n]/)[0]?.trim() || "按预置流程执行自动化";
  const compact = firstSentence
    .replace(/^通过/, "")
    .replace(/^使用/, "")
    .replace(/^基于/, "")
    .replace(/浏览器实例/g, "实例")
    .replace(/本地 txt/gi, "TXT")
    .replace(/\s+/g, " ");

  return compact.length > 30 ? `${compact.slice(0, 28).trim()}...` : compact;
}

function buildAutomationCodeDisplay(
  script: AutomationScriptRecord,
  profiles: BrowserProfile[],
  dualLaunchCodes: DualLaunchCodes,
): string {
  if (script.id === DUAL_INSTANCE_SCRIPT_ID) {
    return `${dualLaunchCodes.primaryCode} / ${dualLaunchCodes.secondaryCode}`;
  }

  switch (script.targetConfig.mode) {
    case "existing": {
      return resolveTargetCode(script.targetConfig.selector, profiles) || "运行时传入";
    }
    case "create": {
      return (
        resolveTargetCode(script.targetConfig.templateSelector, profiles) ||
        "运行时传入"
      );
    }
    case "rotate": {
      const code = resolveTargetCode(script.targetConfig.selector, profiles);
      if (code) {
        return code;
      }
      const selector = script.targetConfig.selector;
      const hasFilter = Boolean(
        normalizeText(selector.profileId) ||
          normalizeText(selector.profileName) ||
          normalizeText(selector.groupId) ||
          selector.keywords.length > 0 ||
          selector.tags.length > 0,
      );
      return hasFilter ? "条件匹配" : "运行时传入";
    }
    default: {
      const selector = parseJSONObjectText(script.selectorText);
      const directCode = normalizeCode(
        typeof selector?.code === "string" ? selector.code : "",
      );
      const launchCode = normalizeCode(
        typeof selector?.launchCode === "string" ? selector.launchCode : "",
      );
      return directCode || launchCode || "运行时传入";
    }
  }
}

function buildAutomationCardPresentation(options: {
  script: AutomationScriptRecord;
  profiles: BrowserProfile[];
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  dualLaunchCodes: DualLaunchCodes;
  dualInstanceRunPayload: Record<string, unknown>;
  dualInstanceRunPayloadText: string;
  dualInstanceRunCurlDemo: string;
}): AutomationCardPresentation {
  const { script } = options;
  const isDualInstanceScript = script.id === DUAL_INSTANCE_SCRIPT_ID;
  const requestPayload = isDualInstanceScript
    ? options.dualInstanceRunPayload
    : buildAutomationRequestPayload(script, options.profiles);
  const requestPayloadText = isDualInstanceScript
    ? options.dualInstanceRunPayloadText
    : buildAutomationRequestPayloadText(requestPayload);
  const requestCurlDemo = isDualInstanceScript
    ? options.dualInstanceRunCurlDemo
    : buildAutomationRunCurlDemo({
        launchBaseUrl: options.launchBaseUrl,
        apiAuthEnabled: options.apiAuthEnabled,
        apiAuthHeader: options.apiAuthHeader,
        payload: requestPayload,
      });
  const cardMode = buildAutomationCardMode(script);
  const resolvedPublicAPI = resolveAutomationScriptPublicAPIConfig(script);

  return {
    key: script.id,
    title: script.name,
    scriptId: script.id,
    scriptType: script.type,
    modeLabel: getAutomationModeLabel(script.type),
    description: buildAutomationShortDescription(script),
    codeDisplay: buildAutomationCodeDisplay(
      script,
      options.profiles,
      options.dualLaunchCodes,
    ),
    primaryActionLabel: cardMode === "skill" ? "Skill" : "cURL",
    primaryActionText:
      cardMode === "skill"
        ? buildAutomationSkillPrompt(script, requestPayload)
        : requestCurlDemo,
    primaryActionSuccessMessage:
      cardMode === "skill" ? "Skill 提示词已复制" : "模拟 cURL 已复制",
    secondaryActionLabel: "JSON",
    secondaryActionText: requestPayloadText,
    secondaryActionSuccessMessage: "请求 JSON 已复制",
    modeToneClass: getAutomationModeToneClass(script.type),
    publicAPIEnabled: resolvedPublicAPI.enabled,
    railClassName: getAutomationCardRailClass(script.id),
  };
}

function buildDualInstanceFallbackPresentation(options: {
  dualLaunchCodes: DualLaunchCodes;
  dualInstanceRunPayloadText: string;
  dualInstanceRunCurlDemo: string;
}): AutomationCardPresentation {
  return {
    key: `${DUAL_INSTANCE_SCRIPT_ID}-fallback`,
    title: "双实例启动与 Runtime 切换",
    scriptType: "launch-api",
    modeLabel: "接口模式",
    description: "启动双实例并切换 Runtime",
    codeDisplay: `${options.dualLaunchCodes.primaryCode} / ${options.dualLaunchCodes.secondaryCode}`,
    primaryActionLabel: "cURL",
    primaryActionText: options.dualInstanceRunCurlDemo,
    primaryActionSuccessMessage: "模拟 cURL 已复制",
    secondaryActionLabel: "JSON",
    secondaryActionText: options.dualInstanceRunPayloadText,
    secondaryActionSuccessMessage: "请求 JSON 已复制",
    modeToneClass: getAutomationModeToneClass("launch-api"),
    publicAPIEnabled: false,
    railClassName: getAutomationCardRailClass(DUAL_INSTANCE_SCRIPT_ID),
  };
}

function collectAvailableLaunchCodes(profiles: BrowserProfile[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const profile of profiles) {
    const code = normalizeCode(profile.launchCode);
    if (!code || seen.has(code)) {
      continue;
    }
    seen.add(code);
    result.push(code);
  }

  return result;
}

function resolveDualLaunchCodes(profiles: BrowserProfile[]): DualLaunchCodes {
  const availableCodes = collectAvailableLaunchCodes(profiles);
  if (availableCodes.length >= 2) {
    return {
      primaryCode: availableCodes[0],
      secondaryCode: availableCodes[1],
    };
  }
  if (availableCodes.length === 1) {
    return {
      primaryCode: availableCodes[0],
      secondaryCode: "BUYER_002",
    };
  }

  return {
    primaryCode: "BUYER_001",
    secondaryCode: "BUYER_002",
  };
}

function buildCurlAuthHeaderLine(
  apiAuthEnabled: boolean,
  apiAuthHeader: string,
): string {
  if (!apiAuthEnabled) {
    return "";
  }
  return `  -H "${apiAuthHeader}: <YOUR_API_KEY>" \\\n`;
}

function buildPersistablePublicAPIConfig(
  script: AutomationScriptRecord,
): AutomationScriptPublicAPIConfig {
  return prepareAutomationScriptPublicAPIConfigForSave({
    ...script,
    publicAPI: resolveAutomationScriptPublicAPIConfig(script),
  });
}

function mergeImportedScripts(
  current: AutomationScriptRecord[],
  imported: AutomationScriptRecord[],
): AutomationScriptRecord[] {
  const deduped = new Map(imported.map((item) => [item.id, item]));
  return [...imported, ...current.filter((item) => !deduped.has(item.id))];
}

export function AutomationPage() {
  const navigate = useNavigate();
  const { launchBaseUrl, apiAuth } = useLaunchContext();
  const [scripts, setScripts] = useState<AutomationScriptRecord[]>([]);
  const [profiles, setProfiles] = useState<BrowserProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [toolboxOpen, setToolboxOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [runModalOpen, setRunModalOpen] = useState(false);
  const [activeRunScript, setActiveRunScript] =
    useState<AutomationScriptRecord | null>(null);
  const [publicApiModalOpen, setPublicApiModalOpen] = useState(false);
  const [publicApiTestFocusTrigger, setPublicApiTestFocusTrigger] = useState(0);
  const [activePublicApiScript, setActivePublicApiScript] =
    useState<AutomationScriptRecord | null>(null);
  const [publicApiSaving, setPublicApiSaving] = useState(false);
  const [createType, setCreateType] =
    useState<AutomationScriptType>("playwright-cdp");
  const [createName, setCreateName] = useState("");
  const [importMode, setImportMode] = useState<ImportMode>("text");
  const [importText, setImportText] = useState("");
  const [remoteURL, setRemoteURL] = useState("");
  const [gitURL, setGitURL] = useState("");
  const [gitRef, setGitRef] = useState("");
  const [gitScriptPath, setGitScriptPath] = useState("");
  const [busyAction, setBusyAction] = useState<"none" | "create" | "import">(
    "none",
  );

  useEffect(() => {
    let disposed = false;

    void fetchAutomationScripts()
      .then((items) => {
        if (!disposed) {
          setScripts(items);
        }
      })
      .catch(() => {
        toast.error("脚本列表加载失败");
      })
      .finally(() => {
        if (!disposed) {
          setLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    let disposed = false;

    void fetchBrowserProfiles()
      .then((items) => {
        if (!disposed) {
          setProfiles(items || []);
        }
      })
      .catch(() => {
        if (!disposed) {
          setProfiles([]);
        }
      });

    return () => {
      disposed = true;
    };
  }, []);

  const openScript = (scriptId: string) => {
    navigate(`/browser/automation/${scriptId}`);
  };

  const handleOpenRunModal = (script: AutomationScriptRecord) => {
    setActiveRunScript(script);
    setRunModalOpen(true);
  };

  const handleOpenPublicApiModal = (
    script: AutomationScriptRecord,
    options?: { focusTest?: boolean },
  ) => {
    setActivePublicApiScript({ ...script });
    if (options?.focusTest) {
      setPublicApiTestFocusTrigger((current) => current + 1);
    } else {
      setPublicApiTestFocusTrigger(0);
    }
    setPublicApiModalOpen(true);
  };

  const updateActivePublicApiConfig = (
    publicAPI: AutomationScriptPublicAPIConfig,
  ) => {
    setActivePublicApiScript((current) =>
      current
        ? {
            ...current,
            publicAPI,
          }
        : current,
    );
  };

  const persistPublicApiScript = async (
    script: AutomationScriptRecord,
    options?: { silentSuccess?: boolean },
  ): Promise<AutomationScriptRecord | null> => {
    setPublicApiSaving(true);
    try {
      const saved = await saveAutomationScript({
        ...script,
        name: script.name.trim(),
        description: script.description.trim(),
        publicAPI: buildPersistablePublicAPIConfig(script),
        updatedAt: new Date().toISOString(),
      });
      setScripts((current) =>
        current.map((item) => (item.id === saved.id ? saved : item)),
      );
      setActivePublicApiScript(saved);
      if (!options?.silentSuccess) {
        toast.success("接口配置已保存");
      }
      return saved;
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "接口配置保存失败";
      toast.error(message);
      return null;
    } finally {
      setPublicApiSaving(false);
    }
  };

  const handlePreparePublicApiInvoke = async (
    publicAPI: AutomationScriptPublicAPIConfig,
  ): Promise<boolean> => {
    if (!activePublicApiScript) {
      return false;
    }

    const nextScript = {
      ...activePublicApiScript,
      publicAPI,
    };
    setActivePublicApiScript(nextScript);
    const saved = await persistPublicApiScript(nextScript, {
      silentSuccess: true,
    });
    return Boolean(saved);
  };

  const handleRefresh = async () => {
    if (refreshing) {
      return;
    }

    setRefreshing(true);
    try {
      const [scriptsResult, profilesResult] = await Promise.allSettled([
        fetchAutomationScripts(),
        fetchBrowserProfiles(),
      ]);

      if (scriptsResult.status === "fulfilled") {
        setScripts(scriptsResult.value);
      } else {
        toast.error("脚本列表刷新失败");
      }

      if (profilesResult.status === "fulfilled") {
        setProfiles(profilesResult.value || []);
      }

      if (
        scriptsResult.status === "fulfilled" &&
        profilesResult.status === "fulfilled"
      ) {
        toast.success("已刷新");
      }
    } finally {
      setRefreshing(false);
    }
  };

  const resetCreateModal = () => {
    setCreateType("playwright-cdp");
    setCreateName("");
  };

  const closeCreateModal = () => {
    if (busyAction !== "none") {
      return;
    }
    setCreateOpen(false);
    resetCreateModal();
  };

  const resetImportModal = () => {
    setImportMode("text");
    setImportText("");
    setRemoteURL("");
    setGitURL("");
    setGitRef("");
    setGitScriptPath("");
  };

  const closeImportModal = () => {
    if (busyAction !== "none") {
      return;
    }
    setImportOpen(false);
    resetImportModal();
  };

  const handleCreate = async () => {
    setBusyAction("create");
    try {
      const draft = createAutomationScriptDraft(createType);
      if (createName.trim()) {
        draft.name = createName.trim();
      }

      const saved = await saveAutomationScript(draft);
      setScripts((current) => [
        saved,
        ...current.filter((item) => item.id !== saved.id),
      ]);
      setCreateOpen(false);
      resetCreateModal();
      toast.success("脚本已创建");
      openScript(saved.id);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本创建失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const handleImport = async () => {
    setBusyAction("import");
    try {
      let imported: AutomationScriptRecord[] = [];
      let failedCount = 0;

      switch (importMode) {
        case "text": {
          imported = [await importAutomationScriptFromText(importText)];
          break;
        }
        case "local-file":
          imported = [await importAutomationScriptFromLocalFile()];
          break;
        case "local-dir":
          imported = [await importAutomationScriptFromLocalDirectory()];
          break;
        case "local-library": {
          const result = await importAutomationScriptFromLocalLibrary();
          imported = result.imported;
          failedCount = result.failed.length;
          break;
        }
        case "remote-url":
          imported = [await importAutomationScriptFromRemote(remoteURL)];
          break;
        case "git":
          imported = [
            await importAutomationScriptFromGit(gitURL, gitRef, gitScriptPath),
          ];
          break;
        default:
          throw new Error("不支持的导入方式");
      }

      if (imported.length === 0) {
        throw new Error("未导入任何脚本");
      }

      setScripts((current) => mergeImportedScripts(current, imported));
      setImportOpen(false);
      resetImportModal();
      if (imported.length === 1 && failedCount === 0) {
        toast.success("脚本已导入");
        openScript(imported[0].id);
      } else {
        toast.success(`已导入 ${imported.length} 个脚本`);
        if (failedCount > 0) {
          toast.warning(`${failedCount} 个脚本包导入失败`);
        }
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本导入失败";
      toast.error(message);
    } finally {
      setBusyAction("none");
    }
  };

  const dualLaunchCodes = resolveDualLaunchCodes(profiles);
  const dualInstanceRunPayload = {
    scriptId: DUAL_INSTANCE_SCRIPT_ID,
    params: {
      browsers: [
        {
          code: dualLaunchCodes.primaryCode,
          skipDefaultStartUrls: true,
        },
        {
          code: dualLaunchCodes.secondaryCode,
          skipDefaultStartUrls: true,
        },
      ],
      timeoutMs: 45000,
    },
  };
  const dualInstanceRunPayloadText = buildAutomationRequestPayloadText(
    dualInstanceRunPayload,
  );
  const dualInstanceRunCurlDemo = buildAutomationRunCurlDemo({
    launchBaseUrl,
    apiAuthEnabled: apiAuth.enabled,
    apiAuthHeader: apiAuth.header,
    payload: dualInstanceRunPayload,
  });
  const hasDualInstanceBaseline = scripts.some(
    (item) => item.id === DUAL_INSTANCE_SCRIPT_ID,
  );
  const orderedScripts = [...scripts].sort((left, right) => {
    if (left.id === DUAL_INSTANCE_SCRIPT_ID) {
      return -1;
    }
    if (right.id === DUAL_INSTANCE_SCRIPT_ID) {
      return 1;
    }
    return 0;
  });
  const scriptCards = orderedScripts.map((script) =>
    buildAutomationCardPresentation({
      script,
      profiles,
      launchBaseUrl,
      apiAuthEnabled: apiAuth.enabled,
      apiAuthHeader: apiAuth.header,
      dualLaunchCodes,
      dualInstanceRunPayload,
      dualInstanceRunPayloadText,
      dualInstanceRunCurlDemo,
    }),
  );
  const cards: AutomationCardPresentation[] = hasDualInstanceBaseline
    ? scriptCards
    : [
        buildDualInstanceFallbackPresentation({
          dualLaunchCodes,
          dualInstanceRunPayloadText,
          dualInstanceRunCurlDemo,
        }),
        ...scriptCards,
      ];
  const scriptMap = new Map(scripts.map((script) => [script.id, script]));

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">
          脚本管理
        </h1>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => void handleRefresh()}
            loading={refreshing}
          >
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <PlusSquare className="h-4 w-4" />
            新建脚本
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setImportOpen(true)}
          >
            <Upload className="h-4 w-4" />
            导入脚本
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setHistoryOpen(true)}
          >
            <History className="h-4 w-4" />
            调用记录
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setToolboxOpen(true)}
          >
            <Wrench className="h-4 w-4" />
            工具箱
          </Button>
        </div>
      </div>

      <section className="rounded-[28px] border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-3 shadow-[var(--shadow-sm)] md:p-4">
        {loading ? (
          <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">
            正在加载脚本列表...
          </div>
        ) : cards.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-14 text-center">
            <div className="text-base font-medium text-[var(--color-text-primary)]">
              还没有脚本
            </div>
            <div className="mt-2 text-sm text-[var(--color-text-muted)]">
              先新建一套脚本，或者导入已有脚本。
            </div>
            <div className="mt-5 flex justify-center gap-2">
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <PlusSquare className="h-4 w-4" />
                新建
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => setImportOpen(true)}
              >
                <Upload className="h-4 w-4" />
                导入
              </Button>
            </div>
          </div>
        ) : (
          <div
            className="grid items-stretch gap-3"
            style={{
              gridTemplateColumns:
                "repeat(auto-fit, minmax(min(100%, max(430px, calc((100% - 36px) / 4))), 1fr))",
            }}
          >
            {cards.map((card) => {
              const scriptId = card.scriptId;
              const onOpen = scriptId ? () => openScript(scriptId) : undefined;
              const script = scriptId ? scriptMap.get(scriptId) : undefined;
              const publicAPIEnabled = script
                ? resolveAutomationScriptPublicAPIConfig(script).enabled
                : false;
              const onRunScript = script && script.type !== "launch-api"
                ? () => handleOpenRunModal(script)
                : undefined;
              const onRunAPI = script
                ? () =>
                    handleOpenPublicApiModal(script, {
                      focusTest: publicAPIEnabled,
                    })
                : undefined;

              return (
                <div key={card.key} className="min-w-0">
                  <AutomationScriptSummaryCard
                    card={card}
                    onOpen={onOpen}
                    onRunScript={onRunScript}
                    onRunAPI={onRunAPI}
                  />
                </div>
              );
            })}
          </div>
        )}
      </section>

      <Modal
        open={createOpen}
        onClose={closeCreateModal}
        title="新建脚本"
        width="460px"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={closeCreateModal}
              disabled={busyAction !== "none"}
            >
              取消
            </Button>
            <Button
              onClick={() => void handleCreate()}
              loading={busyAction === "create"}
            >
              创建
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <FormItem label="脚本名称">
            <Input
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder="例如：接管页面并截图"
            />
          </FormItem>
          <FormItem label="脚本类型">
            <Select
              value={createType}
              options={AUTOMATION_SCRIPT_TYPE_OPTIONS}
              onChange={(event) =>
                setCreateType(event.target.value as AutomationScriptType)
              }
            />
          </FormItem>
        </div>
      </Modal>

      <Modal
        open={importOpen}
        onClose={closeImportModal}
        title="导入脚本"
        width="720px"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={closeImportModal}
              disabled={busyAction !== "none"}
            >
              取消
            </Button>
            <Button
              onClick={() => void handleImport()}
              loading={busyAction === "import"}
            >
              导入
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {[
              { value: "text", label: "文本" },
              { value: "local-file", label: "本地文件" },
              { value: "local-dir", label: "本地目录" },
              { value: "local-library", label: "脚本库" },
              { value: "remote-url", label: "远程 URL" },
              { value: "git", label: "Git" },
            ].map((item) => (
              <Button
                key={item.value}
                size="sm"
                variant={importMode === item.value ? "primary" : "secondary"}
                onClick={() => setImportMode(item.value as ImportMode)}
                disabled={busyAction !== "none"}
              >
                {item.label}
              </Button>
            ))}
          </div>

          {importMode === "text" ? (
            <>
              <div className="text-sm text-[var(--color-text-secondary)]">
                支持导入导出的脚本 JSON，导入后会按草稿保存。
              </div>
              <FormItem label="脚本 JSON">
                <Textarea
                  rows={18}
                  value={importText}
                  onChange={(event) => setImportText(event.target.value)}
                  className="font-mono"
                  placeholder='{"manifest":{"name":"示例脚本"}}'
                />
              </FormItem>
            </>
          ) : null}

          {importMode === "local-file" ? (
            <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-4 py-4 text-sm text-[var(--color-text-secondary)]">
              导入时会弹出文件选择框。支持单个 `.js/.cjs/.mjs` 脚本文件、导出的
              `.json` 模板，或标准 `.zip` 脚本包。`.ts/.cts/.mts` 仅在设置页开启 TypeScript 导入构建后支持。
            </div>
          ) : null}

          {importMode === "local-dir" ? (
            <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-4 py-4 text-sm text-[var(--color-text-secondary)]">
              导入时会弹出目录选择框。适合导入一整套本地脚本目录，或 Git 拉下来的脚本包目录。目录里的 `.ts/.cts/.mts` 入口也需要先在设置页开启 TypeScript 导入构建。
            </div>
          ) : null}

          {importMode === "local-library" ? (
            <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-4 py-4 text-sm text-[var(--color-text-secondary)]">
              导入时会弹出目录选择框。系统会扫描所选目录下的脚本包并批量导入，来源按本地目录记录，后续刷新不走 Git。
            </div>
          ) : null}

          {importMode === "remote-url" ? (
            <div className="space-y-4">
              <div className="text-sm text-[var(--color-text-secondary)]">
                适合导入单个远程脚本文件、导出的脚本 JSON，或标准脚本 ZIP。多文件仓库也可以继续使用 Git 导入；远程 `.ts/.cts/.mts` 同样要求设置页已开启 TypeScript 导入构建。
              </div>
              <FormItem label="远程地址">
                <Input
                  value={remoteURL}
                  onChange={(event) => setRemoteURL(event.target.value)}
                  placeholder="https://example.com/script.cjs"
                />
              </FormItem>
            </div>
          ) : null}

          {importMode === "git" ? (
            <div className="space-y-4">
              <div className="text-sm text-[var(--color-text-secondary)]">
                会先拉取仓库，再把脚本快照导入当前项目。可以只填一个脚本子目录，系统只扫描那个目录；不填时才会按仓库根目录解析。若入口是 `.ts/.cts/.mts`，需要设置页已开启 TypeScript 导入构建。
              </div>
              <FormItem label="仓库地址">
                <Input
                  value={gitURL}
                  onChange={(event) => setGitURL(event.target.value)}
                  placeholder="https://github.com/example/automation-scripts.git"
                />
              </FormItem>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <FormItem label="分支 / Tag / Commit">
                  <Input
                    value={gitRef}
                    onChange={(event) => setGitRef(event.target.value)}
                    placeholder="main"
                  />
                </FormItem>
                <FormItem label="脚本路径">
                  <Input
                    value={gitScriptPath}
                    onChange={(event) => setGitScriptPath(event.target.value)}
                    placeholder="scripts/demo（留空=仓库根目录）"
                  />
                </FormItem>
              </div>
            </div>
          ) : null}
        </div>
      </Modal>

      <AutomationToolboxModal
        open={toolboxOpen}
        onClose={() => setToolboxOpen(false)}
      />
      <AutomationScriptRunModal
        open={runModalOpen}
        script={activeRunScript}
        dirty={false}
        onClose={() => {
          setRunModalOpen(false);
          setActiveRunScript(null);
        }}
      />
      {activePublicApiScript ? (
        <AutomationScriptPublicApiModal
          open={publicApiModalOpen}
          script={activePublicApiScript}
          busy={publicApiSaving}
          launchBaseUrl={launchBaseUrl}
          apiAuthEnabled={apiAuth.enabled}
          apiAuthHeader={apiAuth.header}
          profiles={profiles}
          focusTestTrigger={publicApiTestFocusTrigger}
          onClose={() => {
            if (publicApiSaving) {
              return;
            }
            setPublicApiModalOpen(false);
            setActivePublicApiScript(null);
          }}
          onChange={updateActivePublicApiConfig}
          onBeforeInvoke={handlePreparePublicApiInvoke}
        />
      ) : null}
      <AutomationScriptHistoryModal
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
      />
    </div>
  );
}
