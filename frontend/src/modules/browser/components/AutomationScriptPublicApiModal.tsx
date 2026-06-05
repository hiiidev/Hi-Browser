import { useEffect, useRef, useState } from "react";
import { Copy, Play, Plus, Sparkles, Trash2 } from "lucide-react";
import {
  Button,
  FormItem,
  Input,
  Modal,
  Select,
  Switch,
  Textarea,
  toast,
} from "../../../shared/components";
import {
  invokeAutomationScriptPublicApi,
  type AutomationScriptPublicApiInvokeResult,
} from "../automationScriptApi";
import { AutomationInstanceSelector } from "./AutomationInstanceSelector";
import type { BrowserProfile } from "../types";
import {
  AUTOMATION_SCRIPT_PUBLIC_API_METHOD_OPTIONS,
  applyAutomationScriptPublicAPIVariables,
  buildAutomationScriptPublicAPIPath,
  buildAutomationScriptPublicAPIRequestExample,
  buildAutomationScriptPublicAPIResponseExample,
  collectAutomationScriptPublicAPIVariableValues,
  DUAL_INSTANCE_RUNTIME_SCRIPT_ID,
  isAutomationScriptPublicAPIVariableName,
  normalizeAutomationScriptPublicAPIConfig,
  prepareAutomationScriptPublicAPIConfigForSave,
  resolveAutomationScriptPublicAPIConfig,
  suggestAutomationScriptPublicAPIPath,
  type AutomationScriptPublicAPIConfig,
  type AutomationScriptPublicAPIVariable,
  type AutomationScriptRecord,
} from "../automationScripts";

interface AutomationScriptPublicApiModalProps {
  open: boolean;
  script: AutomationScriptRecord;
  busy?: boolean;
  launchBaseUrl: string;
  apiAuthEnabled: boolean;
  apiAuthHeader: string;
  profiles?: BrowserProfile[];
  focusTestTrigger?: number;
  onClose: () => void;
  onChange: (config: AutomationScriptPublicAPIConfig) => void;
  onBeforeInvoke?: (
    config: AutomationScriptPublicAPIConfig,
  ) => Promise<boolean> | boolean;
}

function parseJSONText(
  text: string,
): { ok: boolean; value: unknown | null; error: string } {
  const sourceText = String(text || "").trim();
  if (!sourceText) {
    return { ok: true, value: null, error: "" };
  }

  try {
    return {
      ok: true,
      value: JSON.parse(sourceText),
      error: "",
    };
  } catch (error: unknown) {
    return {
      ok: false,
      value: null,
      error: error instanceof Error ? error.message : "JSON 解析失败",
    };
  }
}

function safeParseJSONObject(text: string): Record<string, unknown> | null {
  const parsed = parseJSONText(text);
  if (!parsed.ok || !parsed.value || typeof parsed.value !== "object") {
    return null;
  }
  if (Array.isArray(parsed.value)) {
    return null;
  }
  return parsed.value as Record<string, unknown>;
}

function normalizeLaunchCode(value: unknown): string {
  return String(value || "").trim().toUpperCase();
}

function readPublicApiTargetCode(bodyText: string): string {
  const body = safeParseJSONObject(bodyText);
  if (!body) return "";
  return normalizeLaunchCode(body.code || body.launchCode);
}

function readPublicApiParamObject(
  body: Record<string, unknown>,
): Record<string, unknown> {
  if (body.param && typeof body.param === "object" && !Array.isArray(body.param)) {
    return body.param as Record<string, unknown>;
  }
  if (body.params && typeof body.params === "object" && !Array.isArray(body.params)) {
    return body.params as Record<string, unknown>;
  }
  return {};
}

function readPublicApiDualTargetCode(bodyText: string, index: number): string {
  const body = safeParseJSONObject(bodyText);
  if (!body) return "";
  const param = readPublicApiParamObject(body);
  const browsers = Array.isArray(param.browsers)
    ? param.browsers
    : Array.isArray(body.browsers)
      ? body.browsers
      : [];
  const browser = browsers[index];
  if (!browser || typeof browser !== "object" || Array.isArray(browser)) {
    return "";
  }
  return normalizeLaunchCode(
    (browser as Record<string, unknown>).code ||
      (browser as Record<string, unknown>).launchCode,
  );
}

function buildRequestBodyWithTargetCode(
  currentBodyText: string,
  fallbackBodyText: string,
  code: string,
): string {
  const sourceBody =
    safeParseJSONObject(currentBodyText) || safeParseJSONObject(fallbackBodyText) || {};
  const sourceParam =
    sourceBody.param && typeof sourceBody.param === "object" && !Array.isArray(sourceBody.param)
      ? sourceBody.param
      : sourceBody.params && typeof sourceBody.params === "object" && !Array.isArray(sourceBody.params)
        ? sourceBody.params
        : {};
  const nextBody: Record<string, unknown> = {
    ...sourceBody,
    code: normalizeLaunchCode(code),
    param: sourceParam,
  };
  delete nextBody.launchCode;
  delete nextBody.selector;
  delete nextBody.params;

  return JSON.stringify(nextBody, null, 2);
}

function buildRequestBodyWithDualTargetCode(
  currentBodyText: string,
  fallbackBodyText: string,
  index: number,
  code: string,
): string {
  const sourceBody =
    safeParseJSONObject(currentBodyText) || safeParseJSONObject(fallbackBodyText) || {};
  const sourceParam = readPublicApiParamObject(sourceBody);
  const sourceBrowsers = Array.isArray(sourceParam.browsers)
    ? sourceParam.browsers
    : [];
  const nextBrowsers = [...sourceBrowsers];
  const currentBrowser = nextBrowsers[index];
  const nextBrowser =
    currentBrowser && typeof currentBrowser === "object" && !Array.isArray(currentBrowser)
      ? { ...(currentBrowser as Record<string, unknown>) }
      : {};
  nextBrowser.code = normalizeLaunchCode(code);
  delete nextBrowser.launchCode;
  nextBrowsers[index] = nextBrowser;

  const nextBody: Record<string, unknown> = {
    ...sourceBody,
    param: {
      ...sourceParam,
      browsers: nextBrowsers,
    },
  };
  delete nextBody.params;
  delete nextBody.browsers;

  return JSON.stringify(nextBody, null, 2);
}

function buildCurlPreview(
  script: AutomationScriptRecord,
  config: AutomationScriptPublicAPIConfig,
  launchBaseUrl: string,
  apiAuthEnabled: boolean,
  apiAuthHeader: string,
): string {
  const lines = [
    `curl -X ${config.method} ${launchBaseUrl}${buildAutomationScriptPublicAPIPath(config.path)} \\`,
    `  -H "Content-Type: application/json" \\`,
  ];

  if (apiAuthEnabled && apiAuthHeader.trim()) {
    lines.push(`  -H "${apiAuthHeader}: <YOUR_API_KEY>" \\`);
  }

  const requestBody = applyAutomationScriptPublicAPIVariables(
    buildAutomationScriptPublicAPIRequestExample(script, config),
    config.variables,
    collectAutomationScriptPublicAPIVariableValues(config),
  ).bodyText
    .split("\n")
    .map((line, index, all) =>
      index === all.length - 1 ? `  -d '${line}'` : `  -d '${line}`,
    )
    .join("\n");

  lines.push(requestBody);
  return lines.join("\n");
}

function formatInvokeResult(result: AutomationScriptPublicApiInvokeResult): string {
  if (result.bodyJson !== null) {
    try {
      return JSON.stringify(result.bodyJson, null, 2);
    } catch {
      // noop
    }
  }
  return result.bodyText.trim() || "(empty)";
}

async function copyText(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage);
  } catch {
    toast.error("复制失败");
  }
}

export function AutomationScriptPublicApiModal({
  open,
  script,
  busy = false,
  launchBaseUrl,
  apiAuthEnabled,
  apiAuthHeader,
  profiles = [],
  focusTestTrigger = 0,
  onClose,
  onChange,
  onBeforeInvoke,
}: AutomationScriptPublicApiModalProps) {
  const storedConfig = prepareAutomationScriptPublicAPIConfigForSave(script);
  const resolvedConfig = resolveAutomationScriptPublicAPIConfig(script);
  const fullPath = buildAutomationScriptPublicAPIPath(resolvedConfig.path);
  const fullURL = `${launchBaseUrl}${fullPath}`;
  const requestExampleFallback = buildAutomationScriptPublicAPIRequestExample(
    script,
    {
      ...resolvedConfig,
      requestBodyText: "",
    },
  );
  const responseExampleFallback = buildAutomationScriptPublicAPIResponseExample(
    script,
    {
      ...resolvedConfig,
      responseBodyText: "",
    },
  );
  const resolvedRequestBody = applyAutomationScriptPublicAPIVariables(
    resolvedConfig.requestBodyText,
    resolvedConfig.variables,
    collectAutomationScriptPublicAPIVariableValues(resolvedConfig),
  );
  const resolvedRequestBodyText = resolvedRequestBody.bodyText;
  const invalidVariableNames = resolvedConfig.variables
    .filter((variable) => !isAutomationScriptPublicAPIVariableName(variable.name))
    .map((variable) => variable.name);
  const variableError = invalidVariableNames.length
    ? `变量名只能使用字母、数字、下划线，且不能以数字开头：${invalidVariableNames.join(", ")}`
    : resolvedRequestBody.missingRequired.length
      ? `必填变量缺少默认值：${resolvedRequestBody.missingRequired.join(", ")}`
      : "";
  const responseBodyValidation = parseJSONText(resolvedConfig.responseBodyText);
  const requestBodyError =
    resolvedRequestBodyText.trim() && !safeParseJSONObject(resolvedRequestBodyText)
      ? "替换变量后的请求 Body 必须是 JSON 对象"
      : "";
  const responseBodyError =
    resolvedConfig.responseBodyText.trim() && !responseBodyValidation.ok
      ? `响应示例不是合法 JSON：${responseBodyValidation.error}`
      : "";
  const isDualInstanceRuntimeScript = script.id === DUAL_INSTANCE_RUNTIME_SCRIPT_ID;
  const selectedTargetCode = readPublicApiTargetCode(resolvedRequestBodyText);
  const selectedPrimaryTargetCode = readPublicApiDualTargetCode(
    resolvedRequestBodyText,
    0,
  );
  const selectedSecondaryTargetCode = readPublicApiDualTargetCode(
    resolvedRequestBodyText,
    1,
  );
  const targetCodeError =
    isDualInstanceRuntimeScript
      ? selectedPrimaryTargetCode && selectedSecondaryTargetCode
        ? ""
        : "两个实例 Code 必填"
      : selectedTargetCode
        ? ""
        : "实例 Code 必填";

  const [apiKey, setApiKey] = useState("");
  const [invoking, setInvoking] = useState(false);
  const [invokeResult, setInvokeResult] =
    useState<AutomationScriptPublicApiInvokeResult | null>(null);
  const [invokeError, setInvokeError] = useState("");
  const testSectionRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) {
      setInvoking(false);
      setInvokeResult(null);
      setInvokeError("");
    }
  }, [open, script.id]);

  useEffect(() => {
    if (!open || focusTestTrigger <= 0) {
      return;
    }

    const frameId = window.requestAnimationFrame(() => {
      testSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });

    return () => {
      window.cancelAnimationFrame(frameId);
    };
  }, [focusTestTrigger, open]);

  const updateConfig = (patch: Partial<AutomationScriptPublicAPIConfig>) => {
    const nextConfig = normalizeAutomationScriptPublicAPIConfig({
      ...storedConfig,
      ...patch,
    });
    onChange(nextConfig);
  };

  const handleApplySuggestedPath = () => {
    updateConfig({ path: suggestAutomationScriptPublicAPIPath(script) });
  };

  const handleTargetCodeChange = (code: string) => {
    updateConfig({
      requestBodyText: buildRequestBodyWithTargetCode(
        resolvedConfig.requestBodyText,
        requestExampleFallback,
        code,
      ),
    });
  };

  const handleDualTargetCodeChange = (index: number, code: string) => {
    updateConfig({
      requestBodyText: buildRequestBodyWithDualTargetCode(
        resolvedConfig.requestBodyText,
        requestExampleFallback,
        index,
        code,
      ),
    });
  };

  const updateVariable = (
    index: number,
    patch: Partial<AutomationScriptPublicAPIVariable>,
  ) => {
    updateConfig({
      variables: resolvedConfig.variables.map((variable, variableIndex) =>
        variableIndex === index ? { ...variable, ...patch } : variable,
      ),
    });
  };

  const handleAddVariable = () => {
    const existingNames = new Set(
      resolvedConfig.variables.map((variable) => variable.name),
    );
    const baseName = [
      "searchQuery",
      "senderEmail",
      "recipient",
      "mailboxName",
    ].find((name) => !existingNames.has(name));
    updateConfig({
      variables: [
        ...resolvedConfig.variables,
        {
          name: baseName || `variable${resolvedConfig.variables.length + 1}`,
          defaultValue: "",
          description: "",
          required: false,
        },
      ],
    });
  };

  const handleRemoveVariable = (index: number) => {
    updateConfig({
      variables: resolvedConfig.variables.filter(
        (_variable, variableIndex) => variableIndex !== index,
      ),
    });
  };

  const handleInvoke = async () => {
    if (!resolvedConfig.enabled) {
      toast.warning("请先启用对外接口");
      return;
    }
    if (variableError) {
      toast.warning(variableError);
      return;
    }
    if (requestBodyError) {
      toast.warning(requestBodyError);
      return;
    }
    if (responseBodyError) {
      toast.warning(responseBodyError);
      return;
    }
    if (targetCodeError) {
      toast.warning(targetCodeError);
      return;
    }

    setInvoking(true);
    setInvokeError("");
    setInvokeResult(null);

    try {
      if (onBeforeInvoke) {
        const allowed = await onBeforeInvoke(resolvedConfig);
        if (!allowed) {
          return;
        }
      }

      const result = await invokeAutomationScriptPublicApi({
        url: fullURL,
        method: resolvedConfig.method,
        bodyText: resolvedRequestBodyText,
        apiKey,
        authHeader: apiAuthHeader,
        timeoutMs: resolvedConfig.timeoutMs + 10000,
      });
      setInvokeResult(result);
      if (result.ok) {
        toast.success("测试完成");
      } else {
        toast.warning(`接口返回 ${result.status}`);
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "测试失败";
      setInvokeError(message);
      toast.error(message);
    } finally {
      setInvoking(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="对外接口管理"
      width="1100px"
      footer={
        <Button variant="secondary" onClick={onClose}>
          完成
        </Button>
      }
    >
      <div className="space-y-4">
        <div
          ref={testSectionRef}
          className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">
              测试接口
            </div>
            <Button
              type="button"
              size="sm"
              onClick={() => void handleInvoke()}
              loading={invoking}
              disabled={
                busy ||
                !resolvedConfig.enabled ||
                !!variableError ||
                !!requestBodyError ||
                !!responseBodyError ||
                !!targetCodeError
              }
            >
              <Play className="h-4 w-4" />
              发送测试请求
            </Button>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
            <div className="space-y-3">
              <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-text-muted)]">
                  目标地址
                </div>
                <div className="mt-2 break-all text-sm text-[var(--color-text-primary)]">
                  {fullURL}
                </div>
              </div>

              {apiAuthEnabled ? (
                <FormItem label={`API Key (${apiAuthHeader})`}>
                  <Input
                    value={apiKey}
                    onChange={(event) => setApiKey(event.target.value)}
                    placeholder="留空则使用当前应用里的 Launch API Key"
                  />
                </FormItem>
              ) : (
                <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-3 text-sm text-[var(--color-text-secondary)]">
                  当前 Launch API 未启用认证，可以直接测试。
                </div>
              )}
            </div>

            <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  返回结果
                </div>
                {invokeResult ? (
                  <div className="text-xs text-[var(--color-text-muted)]">
                    HTTP {invokeResult.status} {invokeResult.statusText}
                  </div>
                ) : null}
              </div>

              {invokeError ? (
                <div className="mt-3 rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error)]/10 px-3 py-3 text-sm text-[var(--color-text-secondary)]">
                  {invokeError}
                </div>
              ) : null}

              {!invokeError && !invokeResult ? (
                <div className="mt-3 rounded-lg border border-dashed border-[var(--color-border-muted)] px-3 py-8 text-center text-sm text-[var(--color-text-muted)]">
                  发送一次测试请求后，这里显示真实响应。
                </div>
              ) : null}

              {invokeResult ? (
                <pre className="mt-3 overflow-x-auto rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-3 text-xs leading-6 text-[var(--color-text-secondary)]">
                  <code>{formatInvokeResult(invokeResult)}</code>
                </pre>
              ) : null}
            </div>
          </div>
        </div>
        <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <Select
              value={resolvedConfig.method}
              options={AUTOMATION_SCRIPT_PUBLIC_API_METHOD_OPTIONS.map((item) => ({
                value: item.value,
                label: item.label,
              }))}
              onChange={(event) =>
                updateConfig({ method: event.target.value as "POST" })
              }
              className="w-[96px] shrink-0 font-semibold"
              disabled
            />
            <Input
              value={fullURL}
              readOnly
              className="min-w-0 flex-1 font-mono sm:min-w-[280px]"
            />
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() => void copyText(fullURL, "接口地址已复制")}
              disabled={busy}
            >
              <Copy className="h-4 w-4" />
              URL
            </Button>
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() =>
                void copyText(
                  buildCurlPreview(
                    script,
                    resolvedConfig,
                    launchBaseUrl,
                    apiAuthEnabled,
                    apiAuthHeader,
                  ),
                  "curl 已复制",
                )
              }
              disabled={busy}
            >
              <Copy className="h-4 w-4" />
              curl
            </Button>
            <div className="ml-auto flex h-9 items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-sm text-[var(--color-text-secondary)]">
              <span>{resolvedConfig.enabled ? "已启用" : "未启用"}</span>
              <Switch
                checked={resolvedConfig.enabled}
                onChange={(checked) => updateConfig({ enabled: checked })}
                disabled={busy}
              />
            </div>
          </div>

          <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_180px]">
            <FormItem label="Path">
              <div className="flex gap-2">
                <Input
                  value={resolvedConfig.path}
                  onChange={(event) => updateConfig({ path: event.target.value })}
                  placeholder="mail/proton-first-message"
                  className="font-mono"
                  disabled={busy}
                />
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="!h-9 !min-w-[88px] shrink-0 whitespace-nowrap"
                  onClick={handleApplySuggestedPath}
                  disabled={busy}
                >
                  <Sparkles className="h-4 w-4" />
                  推荐
                </Button>
              </div>
              <div className="mt-1 break-all text-xs text-[var(--color-text-muted)]">
                {fullPath}
              </div>
            </FormItem>

            <FormItem label="Timeout">
              <Input
                type="number"
                min={1000}
                max={1800000}
                value={String(resolvedConfig.timeoutMs)}
                onChange={(event) =>
                  updateConfig({ timeoutMs: Number(event.target.value) || 0 })
                }
                disabled={busy}
              />
            </FormItem>
          </div>
        </div>

        <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">
              变量
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handleAddVariable}
              disabled={busy}
            >
              <Plus className="h-4 w-4" />
              新增
            </Button>
          </div>

          {resolvedConfig.variables.length > 0 ? (
            <div className="mt-3 space-y-2">
              {resolvedConfig.variables.map((variable, index) => (
                <div
                  key={`${index}-${variable.name}`}
                  className="grid grid-cols-1 gap-2 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-2 py-2 lg:grid-cols-[180px_minmax(0,1fr)_minmax(0,1fr)_86px_36px]"
                >
                  <Input
                    value={variable.name}
                    onChange={(event) =>
                      updateVariable(index, { name: event.target.value })
                    }
                    placeholder="searchQuery"
                    className="font-mono"
                    disabled={busy}
                  />
                  <Input
                    value={variable.defaultValue}
                    onChange={(event) =>
                      updateVariable(index, { defaultValue: event.target.value })
                    }
                    placeholder="默认值"
                    disabled={busy}
                  />
                  <Input
                    value={variable.description}
                    onChange={(event) =>
                      updateVariable(index, { description: event.target.value })
                    }
                    placeholder="说明"
                    disabled={busy}
                  />
                  <label className="flex h-9 items-center justify-center gap-2 rounded-lg border border-[var(--color-border-muted)] text-sm text-[var(--color-text-secondary)]">
                    <input
                      type="checkbox"
                      checked={variable.required}
                      onChange={(event) =>
                        updateVariable(index, { required: event.target.checked })
                      }
                      disabled={busy}
                    />
                    必填
                  </label>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={() => handleRemoveVariable(index)}
                    disabled={busy}
                    aria-label="删除变量"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <div className="mt-3 rounded-lg border border-dashed border-[var(--color-border-muted)] px-3 py-3 text-sm text-[var(--color-text-muted)]">
              未配置变量
            </div>
          )}

          {variableError ? (
            <p className="mt-2 text-xs text-[var(--color-error)]">
              {variableError}
            </p>
          ) : (
            <p className="mt-2 text-xs text-[var(--color-text-muted)]">
              Body 中使用 <code>{"${name}"}</code>，测试和 curl 会替换为默认值。
            </p>
          )}
        </div>

        {isDualInstanceRuntimeScript ? (
          <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
            <AutomationInstanceSelector
              title="传入实例 1"
              mode="manual"
              modes={["manual"]}
              profiles={profiles}
              selectedCode={selectedPrimaryTargetCode}
              disabled={busy}
              codePlaceholder="例如 BUYER_001"
              onCodeChange={(code) => handleDualTargetCodeChange(0, code)}
            />
            <AutomationInstanceSelector
              title="传入实例 2"
              mode="manual"
              modes={["manual"]}
              profiles={profiles}
              selectedCode={selectedSecondaryTargetCode}
              disabled={busy}
              codePlaceholder="例如 BUYER_002"
              onCodeChange={(code) => handleDualTargetCodeChange(1, code)}
            />
          </div>
        ) : (
          <AutomationInstanceSelector
            title="传入实例"
            mode="manual"
            modes={["manual"]}
            profiles={profiles}
            selectedCode={selectedTargetCode}
            disabled={busy}
            codePlaceholder="例如 BUYER_001"
            onCodeChange={handleTargetCodeChange}
          />
        )}

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                Body 入参
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => updateConfig({ requestBodyText: "" })}
                  disabled={busy}
                >
                  默认
                </Button>
              </div>
            </div>
            <Textarea
              rows={13}
              value={resolvedConfig.requestBodyText}
              onChange={(event) =>
                updateConfig({ requestBodyText: event.target.value })
              }
              className="mt-3 font-mono"
              placeholder={requestExampleFallback}
              disabled={busy}
            />
            {requestBodyError ? (
              <p className="mt-2 text-xs text-[var(--color-error)]">
                {requestBodyError}
              </p>
            ) : null}
          </div>

          <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] px-3 py-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                Response 出参
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => updateConfig({ responseBodyText: "" })}
                  disabled={busy}
                >
                  默认
                </Button>
              </div>
            </div>
            <Textarea
              rows={13}
              value={resolvedConfig.responseBodyText}
              onChange={(event) =>
                updateConfig({ responseBodyText: event.target.value })
              }
              className="mt-3 font-mono"
              placeholder={responseExampleFallback}
              disabled={busy}
            />
            {responseBodyError ? (
              <p className="mt-2 text-xs text-[var(--color-error)]">
                {responseBodyError}
              </p>
            ) : null}
          </div>
        </div>

      </div>
    </Modal>
  );
}
