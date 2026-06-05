import { useEffect, useMemo, useState } from "react";
import { Copy, FileText, FolderOpen, Play } from "lucide-react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  FormItem,
  Input,
  Modal,
  Textarea,
  toast,
} from "../../../shared/components";
import { fetchBrowserProfiles, fetchGroups, openCorePath } from "../api";
import { AutomationInstanceSelector } from "./AutomationInstanceSelector";
import { runAutomationScript } from "../automationScriptApi";
import {
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  applyAutomationScriptPublicAPIVariables,
  collectAutomationScriptPublicAPIVariableValues,
  createAutomationScriptTargetSelector,
  describeAutomationScriptTargetConfig,
  getAutomationScriptTypeLabel,
  normalizeAutomationScriptTargetSelector,
  resolveAutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptRecord,
  type AutomationScriptRunRecord,
  type AutomationScriptTargetSelector,
} from "../automationScripts";
import { TargetSelectorEditor } from "../pages/automationScriptDetail/shared";
import {
  buildGroupOptions,
  buildProfileSuggestions,
} from "../pages/automationScriptDetail/helpers";
import {
  type AutomationDemoSession,
} from "../demoSession";
import { useAutomationDemoSession } from "../hooks/useAutomationDemoSession";
import type { BrowserGroupWithCount, BrowserProfile } from "../types";

type DemoPreparationMode = "select" | "create";

type SelectableProfile = BrowserProfile & {
  launchCode: string;
};

interface DemoCreateDraft {
  profileName: string;
  templateProfileId: string;
}

interface ResultOutputEntry {
  key: string;
  label: string;
  path: string;
}

interface AutomationScriptRunModalProps {
  open: boolean;
  script: AutomationScriptRecord | null;
  dirty?: boolean;
  onClose: () => void;
}

type RunVariableInputs = Record<string, string>;

const DEFAULT_DEMO_CREATE_DRAFT: DemoCreateDraft = {
  profileName: "",
  templateProfileId: "",
};

function validateJsonObjectText(
  text: string,
  label: string,
  required: boolean,
): string {
  const normalized = text.trim();
  if (!normalized) {
    return required ? `${label}不能为空` : "";
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return `${label}必须是 JSON 对象`;
    }
    return "";
  } catch {
    return `${label}不是合法 JSON`;
  }
}

function formatDateTime(value?: string): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatDuration(durationMs?: number): string {
  if (!durationMs || durationMs <= 0) {
    return "-";
  }
  if (durationMs < 1000) {
    return `${durationMs} ms`;
  }
  return `${(durationMs / 1000).toFixed(2)} s`;
}

function parseRunResultOutputs(resultText?: string): ResultOutputEntry[] {
  const normalized = String(resultText || "").trim();
  if (!normalized) {
    return [];
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return [];
    }

    const seen = new Set<string>();
    const outputs: ResultOutputEntry[] = [];

    const addOutput = (key: string, value: string) => {
      const path = value.trim();
      if (!path || seen.has(path)) {
        return;
      }
      seen.add(path);
      outputs.push({
        key,
        label: formatRunResultOutputLabel(key),
        path,
      });
    };

    const collectOutputs = (value: unknown, keyHint = "") => {
      if (!value) {
        return;
      }
      if (typeof value === "string") {
        if (/path$/i.test(keyHint)) {
          addOutput(keyHint, value);
        }
        return;
      }
      if (Array.isArray(value)) {
        if (keyHint === "artifacts") {
          value.forEach((item) => {
            if (typeof item === "string") {
              addOutput(keyHint, item);
            }
          });
          return;
        }
        value.forEach((item) => collectOutputs(item, keyHint));
        return;
      }
      if (typeof value !== "object") {
        return;
      }

      for (const [nestedKey, nestedValue] of Object.entries(
        value as Record<string, unknown>,
      )) {
        collectOutputs(nestedValue, nestedKey);
      }
    };

    collectOutputs(parsed);
    return outputs;
  } catch {
    return [];
  }
}

function formatRunResultOutputLabel(key: string): string {
  switch (key) {
    case "outputPath":
      return "输出文件";
    case "screenshotPath":
      return "截图文件";
    case "artifacts":
      return "导出文件";
    default:
      return key;
  }
}

function formatRunResultOutputName(path: string): string {
  const segments = path.split(/[\\/]/).filter(Boolean);
  return segments[segments.length - 1] || path;
}

function formatRunResultText(resultText?: string): string {
  const normalized = String(resultText || "").trim();
  if (!normalized) {
    return "";
  }

  try {
    return JSON.stringify(JSON.parse(normalized), null, 2);
  } catch {
    return resultText || "";
  }
}

async function copyToClipboard(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}

function buildDemoSelectorText(launchCode: string) {
  return JSON.stringify(
    {
      code: launchCode,
    },
    null,
    2,
  );
}

function normalizeLaunchCode(value?: string): string {
  return String(value || "")
    .trim()
    .toUpperCase();
}

function isPlaceholderSelectorText(text: string): boolean {
  const normalized = text.trim();
  if (!normalized) {
    return true;
  }

  try {
    const parsed = JSON.parse(normalized);
    const code =
      parsed && typeof parsed === "object" && !Array.isArray(parsed)
        ? String((parsed as Record<string, unknown>).code || "")
            .trim()
            .toUpperCase()
        : "";
    return !code || code === "BUYER_001" || code === "DEMO_ABC123";
  } catch {
    return false;
  }
}

function parseJsonObjectText(text: string): Record<string, unknown> {
  const normalized = text.trim();
  if (!normalized) {
    return {};
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }
    return parsed as Record<string, unknown>;
  } catch {
    return {};
  }
}

function mergeJsonObjectValues(
  base: Record<string, unknown>,
  patch: Record<string, unknown>,
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...base };
  Object.entries(patch).forEach(([key, value]) => {
    const baseValue = merged[key];
    if (
      baseValue &&
      typeof baseValue === "object" &&
      !Array.isArray(baseValue) &&
      value &&
      typeof value === "object" &&
      !Array.isArray(value)
    ) {
      merged[key] = mergeJsonObjectValues(
        baseValue as Record<string, unknown>,
        value as Record<string, unknown>,
      );
      return;
    }
    merged[key] = value;
  });
  return merged;
}

function buildPublicAPIVariableInputs(
  config: AutomationScriptPublicAPIConfig,
): RunVariableInputs {
  return collectAutomationScriptPublicAPIVariableValues(config);
}

function buildParamsTextFromPublicAPIRequest(
  config: AutomationScriptPublicAPIConfig,
  values: RunVariableInputs,
  fallbackParamsText: string,
): { paramsText: string; missingRequired: string[]; usedVariables: string[] } {
  const resolvedBody = applyAutomationScriptPublicAPIVariables(
    config.requestBodyText,
    config.variables,
    values,
  );
  const body = parseJsonObjectText(resolvedBody.bodyText);
  const fallbackParams = parseJsonObjectText(fallbackParamsText);
  const requestParams =
    config.requestMode === "params-only"
      ? body
      : body.params && typeof body.params === "object" && !Array.isArray(body.params)
        ? (body.params as Record<string, unknown>)
        : {};
  const params =
    Object.keys(requestParams).length > 0
      ? mergeJsonObjectValues(fallbackParams, requestParams)
      : fallbackParams;

  return {
    paramsText: JSON.stringify(params, null, 2),
    missingRequired: resolvedBody.missingRequired,
    usedVariables: resolvedBody.usedVariables,
  };
}

function isCodeOnlySelectorForLaunchCode(
  text: string,
  launchCode: string,
): boolean {
  const normalizedCode = normalizeLaunchCode(launchCode);
  const normalizedText = text.trim();
  if (!normalizedCode || !normalizedText) {
    return false;
  }

  try {
    const parsed = JSON.parse(normalizedText);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return false;
    }

    const entries = Object.entries(parsed as Record<string, unknown>).filter(
      ([, value]) => {
        if (value == null) {
          return false;
        }
        if (typeof value === "string") {
          return value.trim() !== "";
        }
        if (Array.isArray(value)) {
          return value.length > 0;
        }
        return true;
      },
    );
    if (entries.length !== 1 || entries[0]?.[0] !== "code") {
      return false;
    }

    return normalizeLaunchCode(String(entries[0][1] || "")) === normalizedCode;
  } catch {
    return false;
  }
}

function resolveInitialSelectorText(
  script: AutomationScriptRecord,
  demoSession: AutomationDemoSession,
): string {
  if (
    script.targetConfig.mode !== "manual" &&
    script.targetConfig.mode !== "existing"
  ) {
    return "";
  }
  if (script.targetConfig.mode === "existing") {
    const selectorCode = normalizeLaunchCode(script.targetConfig.selector.code);
    if (selectorCode) {
      return buildDemoSelectorText(selectorCode);
    }
  }
  const currentSelectorText = String(script.selectorText || "");
  if (
    script.type === "playwright-cdp" &&
    isPlaceholderSelectorText(currentSelectorText) &&
    demoSession.launchCode
  ) {
    return buildDemoSelectorText(demoSession.launchCode);
  }
  return currentSelectorText;
}

function resolveRunnableSelectorText(
  script: AutomationScriptRecord,
  currentSelectorText: string,
  demoSession: AutomationDemoSession,
): string {
  if (
    script.targetConfig.mode !== "manual" &&
    script.targetConfig.mode !== "existing"
  ) {
    return currentSelectorText;
  }
  if (
    script.type === "playwright-cdp" &&
    isPlaceholderSelectorText(currentSelectorText) &&
    demoSession.launchCode
  ) {
    return buildDemoSelectorText(demoSession.launchCode);
  }
  return currentSelectorText;
}

function resolveSelectorLaunchCode(text: string): string {
  const normalized = text.trim();
  if (!normalized) {
    return "";
  }

  try {
    const parsed = JSON.parse(normalized);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return "";
    }

    return String((parsed as Record<string, unknown>).code || "")
      .trim()
      .toUpperCase();
  } catch {
    return "";
  }
}

function filterSelectableProfiles(profiles: BrowserProfile[]): SelectableProfile[] {
  return profiles
    .flatMap((profile) => {
      const launchCode = normalizeLaunchCode(profile.launchCode);
      if (!launchCode) {
        return [];
      }
      return [
        {
          ...profile,
          launchCode,
        },
      ];
    })
    .sort((left, right) => {
      if (left.running !== right.running) {
        return left.running ? -1 : 1;
      }
      return left.profileName.localeCompare(right.profileName, "zh-CN");
    });
}

function resolvePreferredProfileId(
  profiles: SelectableProfile[],
  preferredProfileId: string,
  preferredLaunchCode: string,
): string {
  const normalizedProfileId = String(preferredProfileId || "").trim();
  const normalizedCode = normalizeLaunchCode(preferredLaunchCode);
  if (!normalizedProfileId && !normalizedCode) {
    return "";
  }

  if (normalizedProfileId) {
    const matchedByID = profiles.find(
      (profile) => profile.profileId === normalizedProfileId,
    );
    if (matchedByID) {
      return matchedByID.profileId;
    }
  }

  const matchedByCode = profiles.find(
    (profile) => normalizeLaunchCode(profile.launchCode) === normalizedCode,
  );
  if (matchedByCode) {
    return matchedByCode.profileId;
  }

  return "";
}

function buildSelectableProfileOptions(profiles: SelectableProfile[]) {
  return profiles.map((profile) => ({
    value: profile.profileId,
    label: `${profile.launchCode} · ${profile.profileName} · ${formatSelectableProfileStatus(profile)}`,
  }));
}

function formatSelectableProfileStatus(profile: SelectableProfile): string {
  if (profile.running && profile.debugReady && profile.debugPort > 0) {
    return "可连接";
  }
  if (profile.running) {
    return "启动中";
  }
  return "未启动，执行时自动启动";
}

function sortTemplateProfiles(profiles: BrowserProfile[]) {
  return [...profiles].sort((left, right) =>
    left.profileName.localeCompare(right.profileName, "zh-CN"),
  );
}

function buildTemplateProfileOptions(profiles: BrowserProfile[]) {
  return profiles.map((profile) => ({
    value: profile.profileId,
    label: [profile.launchCode || "", profile.profileName || profile.profileId]
      .filter(Boolean)
      .join(" · "),
  }));
}

export function AutomationScriptRunModal({
  open,
  script,
  dirty = false,
  onClose,
}: AutomationScriptRunModalProps) {
  const navigate = useNavigate();
  const [selectorText, setSelectorText] = useState("");
  const [paramsText, setParamsText] = useState("");
  const [variableInputs, setVariableInputs] = useState<RunVariableInputs>({});
  const [running, setRunning] = useState(false);
  const demoBusy = false;
  const [lastRun, setLastRun] = useState<AutomationScriptRunRecord | null>(
    null,
  );
  const [demoMode, setDemoMode] = useState<DemoPreparationMode>("select");
  const [availableProfiles, setAvailableProfiles] = useState<SelectableProfile[]>(
    [],
  );
  const [templateProfiles, setTemplateProfiles] = useState<BrowserProfile[]>([]);
  const [allProfiles, setAllProfiles] = useState<BrowserProfile[]>([]);
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([]);
  const [profilesLoading, setProfilesLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [createDraft, setCreateDraft] = useState<DemoCreateDraft>(
    DEFAULT_DEMO_CREATE_DRAFT,
  );
  const [rotateSelector, setRotateSelector] =
    useState<AutomationScriptTargetSelector>(() =>
      createAutomationScriptTargetSelector(),
    );
  const {
    demoSession,
    setDemoSession,
    reloadDemoSession,
  } = useAutomationDemoSession({ enabled: open });

  const selectedProfile =
    availableProfiles.find((profile) => profile.profileId === selectedProfileId) ||
    null;
  const selectorDetachedFromSelectedProfile =
    demoMode === "select" &&
    !!selectedProfile &&
    !!selectorText.trim() &&
    !isPlaceholderSelectorText(selectorText) &&
    !isCodeOnlySelectorForLaunchCode(selectorText, selectedProfile.launchCode);
  const isDualInstanceRuntimeScript =
    script?.id === DUAL_INSTANCE_RUNTIME_SCRIPT_ID;
  const isManualTargetMode =
    !!script &&
    (script.targetConfig.mode === "manual" || script.targetConfig.mode === "existing");
  const usesStoredTargetConfig =
    !!script && !isManualTargetMode;
  const showsSelectorInput =
    !!script && !usesStoredTargetConfig && !isDualInstanceRuntimeScript;
  const selectedLaunchCode = resolveSelectorLaunchCode(selectorText);
  const publicAPIConfig = useMemo(
    () => (script ? resolveAutomationScriptPublicAPIConfig(script) : null),
    [script],
  );
  const publicAPIVariables = publicAPIConfig?.variables || [];
  const usedPublicAPIVariableNames = useMemo(() => {
    if (!publicAPIConfig) {
      return new Set<string>();
    }
    return new Set(
      applyAutomationScriptPublicAPIVariables(
        publicAPIConfig.requestBodyText,
        publicAPIConfig.variables,
        variableInputs,
      ).usedVariables,
    );
  }, [publicAPIConfig, variableInputs]);
  const hasPublicAPIVariables = publicAPIVariables.length > 0;
  const hasUnusedPublicAPIVariables =
    hasPublicAPIVariables &&
    publicAPIVariables.some(
      (variable) => !usedPublicAPIVariableNames.has(variable.name),
    );
  const codeSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.launchCode,
    (profile) =>
      profile.profileName
        ? `${profile.launchCode || "未设 Code"} · ${profile.profileName}`
        : profile.profileId,
  );
  const profileIdSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.profileId,
    (profile) =>
      profile.launchCode
        ? `${profile.launchCode} · ${profile.profileName || profile.profileId}`
        : profile.profileName || profile.profileId,
  );
  const profileNameSuggestions = buildProfileSuggestions(
    allProfiles,
    (profile) => profile.profileName,
    (profile) =>
      profile.launchCode
        ? `${profile.launchCode} · ${profile.profileId}`
        : profile.profileId,
  );
  const groupOptions = [{ value: "", label: "不限制" }, ...buildGroupOptions(groups)];
  const syncParamsFromPublicAPIVariables = (
    config: AutomationScriptPublicAPIConfig,
    inputs: RunVariableInputs,
    fallbackParamsText: string,
  ) => {
    const resolved = buildParamsTextFromPublicAPIRequest(
      config,
      inputs,
      fallbackParamsText,
    );
    setParamsText(resolved.paramsText);
    return resolved;
  };
  const updateVariableInput = (name: string, value: string) => {
    setVariableInputs((current) => {
      const nextInputs = {
        ...current,
        [name]: value,
      };
      if (publicAPIConfig) {
        syncParamsFromPublicAPIVariables(
          publicAPIConfig,
          nextInputs,
          script?.paramsText || paramsText,
        );
      }
      return nextInputs;
    });
  };
  const updateParamsText = (nextParamsText: string) => {
    setParamsText(nextParamsText);
  };
  const updateRotateSelector = (
    patch: Partial<AutomationScriptTargetSelector>,
  ) => {
    setRotateSelector((current) =>
      normalizeAutomationScriptTargetSelector({
        ...current,
        ...patch,
      }),
    );
  };
  const resolveParamsTextForRun = (): string => {
    if (!publicAPIConfig || !hasPublicAPIVariables) {
      return paramsText;
    }
    return syncParamsFromPublicAPIVariables(
      publicAPIConfig,
      variableInputs,
      script?.paramsText || paramsText,
    ).paramsText;
  };
  const paramsLabel = isDualInstanceRuntimeScript ? "启动配置" : "运行参数";
  const paramsFieldLabel = isDualInstanceRuntimeScript
    ? "浏览器列表 / 启动配置 JSON"
    : "运行参数 JSON";
  const paramsPlaceholder = isDualInstanceRuntimeScript
    ? `{
  "browsers": [
    { "code": "BUYER_001", "skipDefaultStartUrls": true },
    { "code": "BUYER_002", "skipDefaultStartUrls": true }
  ],
  "timeoutMs": 45000
}`
    : '{"startUrls":["https://example.com"]}';

  const syncDemoSessionFromProfile = (
    profile: SelectableProfile,
    actionLabel: string,
  ) => {
    setDemoSession((current) => ({
      ...current,
      profileId: profile.profileId,
      profileName: profile.profileName,
      launchCode: profile.launchCode,
      cdpUrl:
        profile.running && profile.debugReady && profile.debugPort > 0
          ? `http://127.0.0.1:${profile.debugPort}`
          : "",
      debugPort:
        profile.running && profile.debugReady && profile.debugPort > 0
          ? profile.debugPort
          : 0,
      lastAction: actionLabel,
    }));
  };

  const refreshSelectableProfiles = async (
    preferredProfileId = "",
    preferredLaunchCode = "",
    showError = false,
  ) => {
    setProfilesLoading(true);
    try {
      const allProfiles = await fetchBrowserProfiles();
      setAllProfiles(allProfiles);
      const profiles = filterSelectableProfiles(allProfiles);
      const nextSelectedProfileId =
        resolvePreferredProfileId(
          profiles,
          preferredProfileId,
          preferredLaunchCode,
        ) ||
        (selectedProfileId &&
        profiles.some((profile) => profile.profileId === selectedProfileId)
          ? selectedProfileId
          : isManualTargetMode
            ? ""
            : profiles[0]?.profileId || "");
      const nextSelectedProfile =
        profiles.find((profile) => profile.profileId === nextSelectedProfileId) ||
        null;

      setAvailableProfiles(profiles);
      setTemplateProfiles(sortTemplateProfiles(allProfiles));
      setSelectedProfileId(nextSelectedProfileId);
      if (demoMode === "select" && nextSelectedProfile) {
        const keepManualSelector =
          !!selectorText.trim() &&
          !isPlaceholderSelectorText(selectorText) &&
          !isCodeOnlySelectorForLaunchCode(
            selectorText,
            nextSelectedProfile.launchCode,
          );
        const nextSelectorText = buildDemoSelectorText(
          nextSelectedProfile.launchCode,
        );
        if (
          !keepManualSelector &&
          resolveSelectorLaunchCode(selectorText) !==
          nextSelectedProfile.launchCode
        ) {
          setSelectorText(nextSelectorText);
        }
        if (!keepManualSelector) {
          syncDemoSessionFromProfile(nextSelectedProfile, "选择实例");
        }
      }
      setCreateDraft((current) => {
        if (
          current.templateProfileId &&
          allProfiles.some((profile) => profile.profileId === current.templateProfileId)
        ) {
          return current;
        }
        return {
          ...current,
          templateProfileId: allProfiles[0]?.profileId || "",
        };
      });
      if (!profiles.length && !isManualTargetMode) {
        setDemoMode("create");
      }
    } catch (error: unknown) {
      if (showError) {
        const message =
          error instanceof Error ? error.message : "实例列表刷新失败";
        toast.error(message);
      }
    } finally {
      setProfilesLoading(false);
    }
  };

  useEffect(() => {
    if (!open) {
      return;
    }
    let disposed = false;
    void fetchGroups().then(
      (items) => {
        if (!disposed) {
          setGroups(items || []);
        }
      },
      () => {
        if (!disposed) {
          setGroups([]);
        }
      },
    );
    return () => {
      disposed = true;
    };
  }, [open]);

  useEffect(() => {
    if (!open || !script) {
      return;
    }

    const nextDemoSession = reloadDemoSession();
    const nextSelectorText = resolveInitialSelectorText(script, nextDemoSession);
    setSelectorText(nextSelectorText);
    const nextInputs = publicAPIConfig
      ? buildPublicAPIVariableInputs(publicAPIConfig)
      : {};
    setVariableInputs(nextInputs);
    if (publicAPIConfig && publicAPIConfig.variables.length > 0) {
      syncParamsFromPublicAPIVariables(
        publicAPIConfig,
        nextInputs,
        script.paramsText || "",
      );
    } else {
      updateParamsText(script.paramsText || "");
    }
    setLastRun(null);
    setCreateDraft({
      profileName: script.targetConfig.createNameTemplate || "",
      templateProfileId: script.targetConfig.templateSelector.profileId || "",
    });
    setSelectedProfileId(script.targetConfig.selector.profileId || "");
    setRotateSelector(
      normalizeAutomationScriptTargetSelector(script.targetConfig.selector),
    );
    setDemoMode(
      nextDemoSession.launchCode ||
        resolveSelectorLaunchCode(nextSelectorText)
        ? "select"
        : "create",
    );
  }, [open, script, publicAPIConfig]);

  useEffect(() => {
    if (!open || !script) {
      setAvailableProfiles([]);
      setSelectedProfileId("");
      return;
    }

    const nextDemoSession = reloadDemoSession();
    const nextSelectorText = resolveInitialSelectorText(script, nextDemoSession);
    void refreshSelectableProfiles(
      script.targetConfig.selector.profileId || nextDemoSession.profileId,
      resolveSelectorLaunchCode(nextSelectorText) || nextDemoSession.launchCode,
      false,
    );
  }, [open, script, usesStoredTargetConfig]);

  useEffect(() => {
    if (!open || !script || script.type !== "playwright-cdp") {
      return;
    }
    if (usesStoredTargetConfig) {
      return;
    }
    if (demoMode !== "select") {
      return;
    }

    void refreshSelectableProfiles("", demoSession.launchCode, false);
  }, [demoMode, demoSession.launchCode, open, script, usesStoredTargetConfig]);

  const handleClose = () => {
    if (running || demoBusy) {
      return;
    }
    onClose();
  };

  const buildRunTargetInput = (): Record<string, unknown> => {
    if (!script || isManualTargetMode) {
      return {};
    }
    if (script.targetConfig.mode === "create") {
      return {
        templateSelector: createDraft.templateProfileId
          ? { profileId: createDraft.templateProfileId }
          : {},
        createNameTemplate: createDraft.profileName.trim(),
      };
    }
    if (script.targetConfig.mode === "rotate") {
      return rotateSelector as unknown as Record<string, unknown>;
    }
    return {};
  };

  const validateRunTargetInput = (): string => {
    if (!script || isManualTargetMode) {
      return "";
    }
    if (script.targetConfig.mode === "create") {
      if (!createDraft.templateProfileId) {
        return "先选择一个模板实例";
      }
      if (!createDraft.profileName.trim()) {
        return "先输入新实例名称";
      }
    }
    if (script.targetConfig.mode === "rotate") {
      const selector = normalizeAutomationScriptTargetSelector(rotateSelector);
      if (
        !selector.code &&
        !selector.profileId &&
        !selector.profileName &&
        !selector.groupId &&
        selector.keywords.length === 0 &&
        selector.tags.length === 0
      ) {
        return "先填写至少一个轮询条件";
      }
    }
    return "";
  };

  const executeRun = async (nextSelectorText: string, nextParamsText: string) => {
    if (!script) {
      return;
    }

    const runnableSelectorText = usesStoredTargetConfig ? "" : nextSelectorText;
    const launchCode =
      script.type === "playwright-cdp" && !usesStoredTargetConfig
        ? resolveSelectorLaunchCode(runnableSelectorText)
        : "";

    setRunning(true);
    try {
      const run = await runAutomationScript({
        scriptId: script.id,
        selectorText: runnableSelectorText,
        targetInput: buildRunTargetInput(),
        paramsText: nextParamsText,
        useScriptSelector: usesStoredTargetConfig,
        useScriptParams: false,
        launchCode,
        startByCodeBeforeRun:
          script.type === "playwright-cdp" &&
          !usesStoredTargetConfig &&
          !!launchCode,
      });
      setLastRun(run);
      if (run.status === "success") {
        toast.success(run.summary || "脚本执行完成");
      } else {
        toast.error(run.error || run.summary || "脚本执行失败");
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "脚本执行失败";
      toast.error(message);
    } finally {
      setRunning(false);
    }
  };

  const handleSelectedProfileChange = (profileId: string) => {
    setSelectedProfileId(profileId);
    const profile =
      availableProfiles.find((item) => item.profileId === profileId) || null;
    if (!profile) {
      return;
    }

    setSelectorText(buildDemoSelectorText(profile.launchCode));
    syncDemoSessionFromProfile(profile, "选择实例");
  };

  const handleLaunchCodeChange = (code: string) => {
    const launchCode = normalizeLaunchCode(code);
    setSelectorText(launchCode ? buildDemoSelectorText(launchCode) : "");
    const profile =
      availableProfiles.find((item) => item.launchCode === launchCode) || null;
    setSelectedProfileId(profile?.profileId || "");
    if (profile) {
      syncDemoSessionFromProfile(profile, "填写实例 Code");
    }
  };

  const handleSelectorTextChange = (value: string) => {
    setSelectorText(value);
    const launchCode = resolveSelectorLaunchCode(value);
    const profile =
      availableProfiles.find((item) => item.launchCode === launchCode) || null;
    setSelectedProfileId(profile?.profileId || "");
    if (profile) {
      syncDemoSessionFromProfile(profile, "填写 selector");
    }
  };

  const handleRestoreSelectedProfileSelector = () => {
    if (!selectedProfile) {
      return;
    }

    setSelectorText(buildDemoSelectorText(selectedProfile.launchCode));
    syncDemoSessionFromProfile(selectedProfile, "选择实例");
  };

  const handleRun = async () => {
    if (!script) {
      return;
    }

    let nextSelectorText = usesStoredTargetConfig
      ? ""
      : resolveRunnableSelectorText(
          script,
          selectorText,
          demoSession,
        );
    const selectorError = usesStoredTargetConfig
      ? ""
      : validateJsonObjectText(
          nextSelectorText,
          "目标选择器",
          script.type === "launch-api" &&
            !usesStoredTargetConfig &&
            !isDualInstanceRuntimeScript,
        );
    if (selectorError) {
      toast.warning(selectorError);
      return;
    }

    const nextParamsText = resolveParamsTextForRun();
    const paramsError = validateJsonObjectText(nextParamsText, paramsLabel, false);
    if (paramsError) {
      toast.warning(paramsError);
      return;
    }

    const targetInputError = validateRunTargetInput();
    if (targetInputError) {
      toast.warning(targetInputError);
      return;
    }

    if (
      script.type === "playwright-cdp" &&
      !usesStoredTargetConfig &&
      isPlaceholderSelectorText(nextSelectorText)
    ) {
      if (demoMode === "select" && selectedProfile) {
        nextSelectorText = buildDemoSelectorText(selectedProfile.launchCode);
        setSelectorText(nextSelectorText);
        syncDemoSessionFromProfile(selectedProfile, "选择实例");
        toast.success("已自动回填所选实例 selector");
      } else {
        toast.warning(
          demoMode === "create"
            ? "先创建一个实例，或填入可用 code"
            : "先选择实例，或填入可用 Code",
        );
        return;
      }
    }

    if (nextSelectorText !== selectorText) {
      setSelectorText(nextSelectorText);
    }
    if (
      script.type === "playwright-cdp" &&
      !usesStoredTargetConfig &&
      demoMode === "select" &&
      selectedProfile &&
      !selectorDetachedFromSelectedProfile
    ) {
      syncDemoSessionFromProfile(selectedProfile, "选择实例");
    }

    await executeRun(nextSelectorText, nextParamsText);
  };

  const handlePrimaryAction = async () => {
    if (!script) {
      return;
    }
    await handleRun();
  };

  const handleOpenScriptDetail = () => {
    if (!script || running || demoBusy) {
      return;
    }
    onClose();
    navigate(`/browser/automation/${script.id}`);
  };

  const handleOpenOutputPath = async (path: string) => {
    try {
      await openCorePath(path);
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "打开目录失败";
      toast.error(message);
    }
  };

  if (!script) {
    return null;
  }

  const launchApiExecutable = script.status !== "disabled";
  const showDemoProfilePicker =
    script.type === "playwright-cdp" && !usesStoredTargetConfig;
  const selectableProfileOptions = buildSelectableProfileOptions(availableProfiles);
  const templateProfileOptions = buildTemplateProfileOptions(templateProfiles);
  const resultOutputs = parseRunResultOutputs(lastRun?.resultText);
  const formattedResultText = formatRunResultText(lastRun?.resultText);

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="执行脚本"
      width="880px"
      footer={
        <>
          <Button
            variant="secondary"
            onClick={handleClose}
            disabled={running || demoBusy}
          >
            关闭
          </Button>
          <Button
            onClick={() => void handlePrimaryAction()}
            loading={running}
            disabled={!launchApiExecutable || demoBusy}
          >
            <Play className="h-4 w-4" />
            立即执行
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-muted)] pb-3">
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <div className="max-w-[26rem] truncate text-sm font-semibold text-[var(--color-text-primary)]">
                {script.name}
              </div>
              <span className="text-xs text-[var(--color-text-muted)]">
                {formatDateTime(script.updatedAt)}
              </span>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
                <Badge
                  variant={script.type === "launch-api" ? "info" : "default"}
                  size="sm"
                >
                  {getAutomationScriptTypeLabel(script.type)}
                </Badge>
                <Badge
                  variant={
                    script.status === "ready"
                      ? "success"
                      : script.status === "disabled"
                        ? "default"
                        : "warning"
                  }
                  size="sm"
                  dot
                >
                  {script.status === "ready"
                    ? "可用"
                    : script.status === "disabled"
                      ? "停用"
                      : "草稿"}
                </Badge>
            </div>
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleOpenScriptDetail}
            disabled={running || demoBusy}
          >
            <FileText className="h-4 w-4" />
            脚本详情
          </Button>
        </div>

        {dirty && (
          <div className="rounded-xl border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-4 py-3 text-sm text-[var(--color-text-secondary)]">
            {isDualInstanceRuntimeScript
              ? "当前详情页还有未保存修改。本次执行只使用弹窗里的启动配置，不会自动保存页面内容。"
              : "当前详情页还有未保存修改。本次执行只使用弹窗里的 selector / params，不会自动保存页面内容。"}
          </div>
        )}

        {usesStoredTargetConfig && (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3 text-sm text-[var(--color-text-secondary)]">
            <div>
              {describeAutomationScriptTargetConfig(script.targetConfig)}
            </div>
            <div className="mt-2 text-xs text-[var(--color-text-muted)]">
              本次执行沿用脚本配置的实例策略，只填写本策略需要的执行配置。
            </div>
          </div>
        )}

        {showDemoProfilePicker && isManualTargetMode ? (
          <AutomationInstanceSelector
            title="传入实例"
            mode="manual"
            modes={["manual"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            selectedCode={selectedLaunchCode}
            selectedProfileId={selectedProfileId}
            profileOptions={selectableProfileOptions}
            selectPlaceholder="暂无可选实例"
            codePlaceholder="例如 BUYER_001"
            onCodeChange={handleLaunchCodeChange}
            onSelectProfile={handleSelectedProfileChange}
            extra={
              selectorDetachedFromSelectedProfile ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                  <span>当前 selector 已手动修改，执行以下方 JSON 为准。</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleRestoreSelectedProfileSelector}
                    disabled={running || demoBusy || !selectedProfile}
                  >
                    恢复实例联动
                  </Button>
                </div>
              ) : null
            }
          />
        ) : null}

        {script.targetConfig.mode === "create" ? (
          <AutomationInstanceSelector
            title="模板创建"
            mode="create"
            modes={["create"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            createName={createDraft.profileName}
            templateProfileId={createDraft.templateProfileId}
            templateOptions={templateProfileOptions}
            templatePlaceholder="暂无模板"
            onCreateNameChange={(profileName) =>
              setCreateDraft((current) => ({
                ...current,
                profileName,
              }))
            }
            onTemplateChange={(templateProfileId) =>
              setCreateDraft((current) => ({
                ...current,
                templateProfileId,
              }))
            }
          />
        ) : null}

        {script.targetConfig.mode === "rotate" ? (
          <AutomationInstanceSelector
            title="条件轮询"
            mode="rotate"
            modes={["rotate"]}
            disabled={running || demoBusy}
            extra={
              <TargetSelectorEditor
                selector={rotateSelector}
                onChange={updateRotateSelector}
                codeSuggestions={codeSuggestions}
                profileIdSuggestions={profileIdSuggestions}
                profileNameSuggestions={profileNameSuggestions}
                groupOptions={groupOptions}
                disabled={running || demoBusy}
              />
            }
          />
        ) : null}

        {showDemoProfilePicker && !isManualTargetMode && demoMode === "select" ? (
          <AutomationInstanceSelector
            title="实例选择"
            mode="select"
            modes={["select"]}
            loading={profilesLoading}
            disabled={running || demoBusy}
            selectedProfileId={selectedProfileId}
            profileOptions={selectableProfileOptions}
            selectPlaceholder="暂无可选实例"
            hint="也可在下方手动填 selector。"
            onSelectProfile={handleSelectedProfileChange}
            extra={
              selectorDetachedFromSelectedProfile ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
                  <span>当前 selector 已手动修改，执行以下方 JSON 为准。</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleRestoreSelectedProfileSelector}
                    disabled={running || demoBusy}
                  >
                    恢复实例联动
                  </Button>
                </div>
              ) : null
            }
          />
        ) : null}

        {script.status === "disabled" ? (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-4 text-sm text-[var(--color-text-secondary)]">
            该脚本当前处于停用状态，先把状态切回可用再执行。
          </div>
        ) : (
          <div className="space-y-3">
              {hasPublicAPIVariables ? (
                <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-[var(--color-text-primary)]">
                      接口变量
                    </div>
                    {hasUnusedPublicAPIVariables ? (
                      <div className="text-xs text-[var(--color-text-muted)]">
                        未引用变量不生效
                      </div>
                    ) : null}
                  </div>
                  <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                    {publicAPIVariables.map((variable) => (
                      <FormItem key={variable.name} label={variable.name}>
                        <Input
                          value={variableInputs[variable.name] || ""}
                          onChange={(event) =>
                            updateVariableInput(variable.name, event.target.value)
                          }
                          placeholder={variable.description || variable.defaultValue}
                          className="h-10 rounded-lg"
                          disabled={running || demoBusy}
                        />
                      </FormItem>
                    ))}
                  </div>
                </div>
              ) : null}

            <div
              className={
                showsSelectorInput
                  ? "grid grid-cols-1 gap-3 xl:grid-cols-2"
                  : "grid grid-cols-1 gap-3"
              }
            >
              {showsSelectorInput && (
                <FormItem label="目标选择器 JSON">
                  <Textarea
                    rows={hasPublicAPIVariables ? 6 : 9}
                    value={selectorText}
                    onChange={(event) => handleSelectorTextChange(event.target.value)}
                    className="font-mono text-xs"
                    placeholder='{"code":"DEMO_ABC123"}'
                    disabled={running || demoBusy}
                  />
                </FormItem>
              )}

              <FormItem label={paramsFieldLabel}>
                <Textarea
                  rows={hasPublicAPIVariables ? 6 : 9}
                  value={paramsText}
                  onChange={(event) => updateParamsText(event.target.value)}
                  className="font-mono text-xs"
                  placeholder={paramsPlaceholder}
                  disabled={running || demoBusy}
                />
              </FormItem>
            </div>
          </div>
        )}

        {lastRun && (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <Badge
                  variant={lastRun.status === "success" ? "success" : "error"}
                  size="sm"
                  dot
                >
                  {lastRun.status === "success" ? "执行成功" : "执行失败"}
                </Badge>
                <span className="text-sm text-[var(--color-text-primary)]">
                  {lastRun.summary || "执行已完成"}
                </span>
              </div>
              <div className="text-xs text-[var(--color-text-muted)]">
                {formatDateTime(lastRun.startedAt)} ·{" "}
                {formatDuration(lastRun.durationMs)}
              </div>
            </div>

            {lastRun.error && (
              <div className="mt-3 break-all text-sm text-[var(--color-error)]">
                {lastRun.error}
              </div>
            )}

            {lastRun.resultText && (
              <div className="mt-4 space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <div className="text-xs text-[var(--color-text-muted)]">
                    结果输出
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() =>
                      void copyToClipboard(formattedResultText, "执行结果已复制")
                    }
                  >
                    <Copy className="h-3.5 w-3.5" />
                    复制结果
                  </Button>
                </div>
                <Textarea
                  rows={10}
                  value={formattedResultText}
                  readOnly
                  className="font-mono"
                />
                {resultOutputs.length > 0 && (
                  <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
                    <div className="space-y-2">
                      {resultOutputs.map((output) => (
                        <div
                          key={`${output.key}-${output.path}`}
                          className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-2"
                        >
                          <div className="min-w-0 flex-1">
                            <div className="text-sm text-[var(--color-text-primary)]">
                              {output.label} · {formatRunResultOutputName(output.path)}
                            </div>
                            <div className="mt-1 break-all text-xs text-[var(--color-text-muted)]">
                              {output.path}
                            </div>
                          </div>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => void handleOpenOutputPath(output.path)}
                          >
                            <FolderOpen className="h-3.5 w-3.5" />
                            打开文件夹
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}
