import { Button, Select } from "@redis-ui/components";
import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import styled from "styled-components";
import {
  DialogActions,
  DialogBody,
  DialogCard,
  DialogCloseButton,
  DialogError,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  Field,
  FormGrid,
  TextInput,
} from "../../components/afs-kit";
import { getControlPlaneURL } from "../../foundation/api/afs";
import {
  useCreateControlPlaneTokenMutation,
  useCreateWorkspaceCompositionAPIKeyMutation,
  useWorkspaceComposition,
  useWorkspaceCompositions,
} from "../../foundation/hooks/use-afs";
import type {
  AFSMCPCapability,
  AFSMCPProfile,
  AFSMCPToken,
  AFSMCPTokenMountCapability,
  AFSWorkspaceCompositionMount,
} from "../../foundation/types/afs";

type Scope = "workspace" | "control-plane";

type CreatedKey = {
  token: AFSMCPToken;
  scope: Scope;
  workspaceName: string;
};

/** Capability ladder shown across MCP and CLI. */
type UnifiedCapability =
  | "read"
  | "read-write"
  | "read-write-checkpoints"
  | "admin";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  initialWorkspaceId?: string;
  initialScope?: Scope;
};

export function CreateAPIKeyDialog({
  isOpen,
  onClose,
  initialWorkspaceId,
  initialScope,
}: Props) {
  const compositionsQuery = useWorkspaceCompositions(isOpen);
  const createWorkspaceKey = useCreateWorkspaceCompositionAPIKeyMutation();
  const createControlPlaneToken = useCreateControlPlaneTokenMutation();

  const [scope, setScope] = useState<Scope>(initialScope ?? "workspace");
  const [workspaceId, setWorkspaceId] = useState("");
  const [name, setName] = useState("");
  const [capability, setCapability] = useState<UnifiedCapability>("read-write");
  const [expiry, setExpiry] = useState("7d");
  const [createdKey, setCreatedKey] = useState<CreatedKey | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [controlPlaneAck, setControlPlaneAck] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  // Per-mount overrides; key = volumeId. Empty means "inherit from `capability`
  // (or 'read' when the mount is manifest-readonly)".
  const [mountOverrides, setMountOverrides] = useState<Record<string, UnifiedCapability>>({});

  const compositions = useMemo(
    () =>
      (compositionsQuery.data ?? [])
        .slice()
        .sort((left, right) => left.name.localeCompare(right.name)),
    [compositionsQuery.data],
  );
  const compositionDetailQuery = useWorkspaceComposition(
    workspaceId,
    isOpen && scope === "workspace" && workspaceId !== "",
  );
  const mounts: AFSWorkspaceCompositionMount[] =
    compositionDetailQuery.data?.mounts ?? [];

  useEffect(() => {
    if (!isOpen) return;
    setCreatedKey(null);
    setFormError(null);
    setName("");
    setCapability("read-write");
    setExpiry("7d");
    setControlPlaneAck(false);
    setMountOverrides({});
    setScope(initialScope ?? "workspace");
    const fallback = compositions[0]?.id ?? "";
    const match = compositions.find(
      (composition) => composition.id === initialWorkspaceId,
    )?.id;
    setWorkspaceId(match ?? fallback);
  }, [isOpen, initialScope, initialWorkspaceId, compositions]);

  // Reset per-mount overrides when the workspace changes so we don't carry
  // stale volume ids from the previous composition.
  useEffect(() => {
    setMountOverrides({});
  }, [workspaceId]);

  const selected = compositions.find((c) => c.id === workspaceId) ?? null;
  const effectiveMountCap = (mount: AFSWorkspaceCompositionMount): UnifiedCapability =>
    mount.readonly
      ? "read"
      : (mountOverrides[mount.volumeId] ?? capability);
  const pending =
    createWorkspaceKey.isPending || createControlPlaneToken.isPending;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setFormError(null);
    try {
      if (scope === "control-plane") {
        if (!controlPlaneAck) {
          setFormError(
            "Confirm you understand control-plane keys grant account-wide admin.",
          );
          return;
        }
        const token = await createControlPlaneToken.mutateAsync({
          name: name.trim() || undefined,
          expiresAt: expiryValueToTimestamp(expiry),
        });
        setCreatedKey({
          token,
          scope: "control-plane",
          workspaceName: "control plane",
        });
        return;
      }
      if (selected == null) return;
      const mcpCapability = unifiedToMCP(capability);
      const mountCapabilities: AFSMCPTokenMountCapability[] = mounts.map(
        (mount) => ({
          volumeId: mount.volumeId,
          capability: unifiedToMCP(effectiveMountCap(mount)),
        }),
      );
      const token = await createWorkspaceKey.mutateAsync({
        workspaceId: selected.id,
        name: name.trim() || undefined,
        profile: profileForCapability(mcpCapability),
        capability: mcpCapability,
        mountCapabilities,
        expiresAt: expiryValueToTimestamp(expiry),
      });
      setCreatedKey({
        token,
        scope: "workspace",
        workspaceName: token.workspaceName ?? selected.name,
      });
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to create API key.",
      );
    }
  }

  function handleClose() {
    if (pending) return;
    onClose();
  }

  function copy(text: string, label: string) {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(label);
      window.setTimeout(() => setCopied(null), 2000);
    });
  }

  if (!isOpen) return null;

  const createdSnippet = createdKey ? buildCreatedSnippet(createdKey) : null;
  const createdLoginSnippet =
    createdKey && createdKey.scope === "workspace"
      ? buildCLILoginSnippet(
          createdKey.token.token ?? "",
          createdKey.workspaceName,
        )
      : null;
  const createdTokenString = createdKey?.token.token ?? "";

  const submitDisabled = (() => {
    if (pending) return true;
    if (scope === "workspace" && selected == null) return true;
    if (scope === "control-plane" && !controlPlaneAck) return true;
    return false;
  })();

  const submitLabel = (() => {
    if (createdKey) return null;
    if (pending) return "Creating...";
    if (scope === "control-plane") return "Issue control-plane key";
    return "Create workspace key";
  })();

  return (
    <DialogOverlay
      onClick={(event) => {
        if (event.target === event.currentTarget) handleClose();
      }}
    >
      <DialogCard>
        <DialogHeader>
          <div>
            <DialogTitle>
              {createdKey ? "API key created" : "Create API key"}
            </DialogTitle>
            <DialogBody>
              {createdKey
                ? "Copy the key below. It's shown once — store it safely."
                : "Pick what this key reaches: one Agent Workspace, or your whole account."}
            </DialogBody>
          </div>
          <DialogCloseButton onClick={handleClose}>&times;</DialogCloseButton>
        </DialogHeader>

        {createdKey ? (
          <CreatedPanel>
            <FieldBlock>
              <FieldLabel>API key</FieldLabel>
              <CodeBlock>{createdTokenString || "(not returned)"}</CodeBlock>
              <InlineActionsRight>
                <Button
                  size="small"
                  variant="secondary-fill"
                  onClick={() =>
                    createdTokenString && copy(createdTokenString, "key")
                  }
                >
                  {copied === "key" ? "Copied!" : "Copy key"}
                </Button>
              </InlineActionsRight>
            </FieldBlock>

            {createdSnippet ? (
              <FieldBlock>
                <FieldLabel>MCP client config</FieldLabel>
                <CodeBlock>{createdSnippet}</CodeBlock>
                <InlineActionsRight>
                  <Button
                    size="small"
                    variant="secondary-fill"
                    onClick={() => copy(createdSnippet, "snippet")}
                  >
                    {copied === "snippet" ? "Copied!" : "Copy snippet"}
                  </Button>
                </InlineActionsRight>
              </FieldBlock>
            ) : null}

            {createdLoginSnippet ? (
              <FieldBlock>
                <FieldLabel>CLI login + mount</FieldLabel>
                <CodeBlock>{createdLoginSnippet}</CodeBlock>
                <InlineActionsRight>
                  <Button
                    size="small"
                    variant="secondary-fill"
                    onClick={() => copy(createdLoginSnippet, "cli")}
                  >
                    {copied === "cli" ? "Copied!" : "Copy CLI"}
                  </Button>
                </InlineActionsRight>
              </FieldBlock>
            ) : null}

            <DialogActions style={{ justifyContent: "flex-end", marginTop: 8 }}>
              <Button size="medium" onClick={handleClose}>
                Done
              </Button>
            </DialogActions>
          </CreatedPanel>
        ) : (
          <FormGrid onSubmit={submit}>
            <FieldGroup>
              <FieldLabel>Scope</FieldLabel>
              <OptionRow>
                <OptionTile $selected={scope === "workspace"}>
                  <input
                    type="radio"
                    checked={scope === "workspace"}
                    onChange={() => setScope("workspace")}
                  />
                  <OptionLabel>
                    <OptionName>Workspace</OptionName>
                    <OptionHint>
                      Bound to one Agent Workspace — same key works for MCP
                      clients and the CLI.
                    </OptionHint>
                  </OptionLabel>
                </OptionTile>
                <OptionTile $selected={scope === "control-plane"}>
                  <input
                    type="radio"
                    checked={scope === "control-plane"}
                    onChange={() => setScope("control-plane")}
                  />
                  <OptionLabel>
                    <OptionName>Control plane</OptionName>
                    <OptionHint>
                      Account-wide admin — manage workspaces + mint scoped
                      keys.
                    </OptionHint>
                  </OptionLabel>
                </OptionTile>
              </OptionRow>
            </FieldGroup>

            {scope === "workspace" ? (
              <Field>
                Agent Workspace
                <Select
                  options={
                    compositions.length === 0
                      ? [{ value: "", label: "No Agent Workspaces yet" }]
                      : compositions.map((composition) => ({
                          value: composition.id,
                          label: composition.name,
                        }))
                  }
                  value={workspaceId}
                  onChange={(next) => setWorkspaceId(next)}
                  disabled={compositions.length === 0}
                />
              </Field>
            ) : null}

            <Field>
              Name
              <TextInput
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={
                  scope === "control-plane"
                    ? "e.g. dev laptop, staging agent"
                    : "Codex on Rowan's Mac"
                }
              />
            </Field>

            {scope === "workspace" ? (
              mounts.length > 0 ? (
                <FieldGroup>
                  <FieldLabel>Per-volume access</FieldLabel>
                  <MountList>
                    {mounts.map((mount) => {
                      const cap = effectiveMountCap(mount);
                      const lockedReadonly = mount.readonly;
                      return (
                        <MountRow key={mount.volumeId}>
                          <MountInfo>
                            <MountPath>
                              {mount.mountPath || "/"}
                            </MountPath>
                            <MountVolume>
                              {mount.volumeName ?? mount.volumeId}
                              {lockedReadonly ? " · manifest readonly" : ""}
                            </MountVolume>
                          </MountInfo>
                          <MountSelect>
                            <Select
                              aria-label={`Capability for ${
                                mount.volumeName ?? mount.volumeId
                              }`}
                              options={
                                lockedReadonly
                                  ? [{ value: "read", label: "Read" }]
                                  : [
                                      { value: "read", label: "Read" },
                                      {
                                        value: "read-write",
                                        label: "Read + write",
                                      },
                                      {
                                        value: "read-write-checkpoints",
                                        label: "Read + write + checkpoints",
                                      },
                                    ]
                              }
                              value={cap}
                              onChange={(next) =>
                                setMountOverrides((prev) => ({
                                  ...prev,
                                  [mount.volumeId]: next as UnifiedCapability,
                                }))
                              }
                              disabled={lockedReadonly}
                            />
                          </MountSelect>
                        </MountRow>
                      );
                    })}
                  </MountList>
                  <MountHint>
                    Each mount carries its own permission on this key. The
                    workspace manifest is the upper bound — readonly mounts
                    stay readonly.
                  </MountHint>
                </FieldGroup>
              ) : (
                <Field>
                  Capability
                  <Select
                    options={[
                      { value: "read", label: "Read" },
                      { value: "read-write", label: "Read + write" },
                      {
                        value: "read-write-checkpoints",
                        label: "Read + write + checkpoints",
                      },
                    ]}
                    value={capability}
                    onChange={(next) =>
                      setCapability(next as UnifiedCapability)
                    }
                  />
                </Field>
              )
            ) : null}

            <Field>
              Expiry
              <Select
                options={[
                  { value: "24h", label: "24 hours" },
                  { value: "7d", label: "7 days" },
                  { value: "30d", label: "30 days" },
                  { value: "never", label: "No expiry" },
                ]}
                value={expiry}
                onChange={(next) => setExpiry(next)}
              />
            </Field>

            <ToolPreview>
              {toolListForKey(scope, capability).map((tool) => (
                <ToolChip key={tool}>{tool}</ToolChip>
              ))}
            </ToolPreview>

            {scope === "control-plane" ? (
              <ControlPlaneNotice>
                <label>
                  <input
                    type="checkbox"
                    checked={controlPlaneAck}
                    onChange={(event) =>
                      setControlPlaneAck(event.target.checked)
                    }
                  />
                  <span>
                    I understand control-plane keys grant account-wide
                    admin — they can mint, revoke, and manage every other
                    key.
                  </span>
                </label>
              </ControlPlaneNotice>
            ) : null}

            {formError ? <DialogError role="alert">{formError}</DialogError> : null}

            <DialogActions style={{ justifyContent: "flex-end" }}>
              <Button
                size="medium"
                type="button"
                variant="secondary-fill"
                onClick={handleClose}
                disabled={pending}
              >
                Cancel
              </Button>
              <Button type="submit" size="medium" disabled={submitDisabled}>
                {submitLabel}
              </Button>
            </DialogActions>
          </FormGrid>
        )}
      </DialogCard>
    </DialogOverlay>
  );
}

/** Back-compat alias — existing consumers can keep the old name. */
export const CreateMCPAccessDialog = CreateAPIKeyDialog;

function expiryValueToTimestamp(value: string) {
  if (value === "never") return undefined;
  const now = Date.now();
  switch (value) {
    case "24h":
      return new Date(now + 24 * 60 * 60 * 1000).toISOString();
    case "30d":
      return new Date(now + 30 * 24 * 60 * 60 * 1000).toISOString();
    case "7d":
    default:
      return new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString();
  }
}

function profileForCapability(capability: AFSMCPCapability): AFSMCPProfile {
  switch (capability) {
    case "ro":
      return "workspace-ro";
    case "rw-checkpoint":
      return "workspace-rw-checkpoint";
    case "admin":
      return "admin-rw";
    case "rw":
    default:
      return "workspace-rw";
  }
}

function unifiedToMCP(value: UnifiedCapability): AFSMCPCapability {
  switch (value) {
    case "read":
      return "ro";
    case "read-write-checkpoints":
      return "rw-checkpoint";
    case "admin":
      return "admin";
    case "read-write":
    default:
      return "rw";
  }
}

function toolListForKey(scope: Scope, capability: UnifiedCapability): string[] {
  if (scope === "control-plane") {
    return [
      "workspace_list",
      "workspace_get",
      "workspace_create",
      "workspace_fork",
      "workspace_delete",
      "checkpoint_list",
      "checkpoint_create",
      "checkpoint_restore",
      "mcp_token_issue",
      "mcp_token_revoke",
    ];
  }
  switch (capability) {
    case "read":
      return ["file_read", "file_lines", "file_list", "file_glob", "file_grep"];
    case "read-write-checkpoints":
      return [
        "file_read",
        "file_write",
        "file_patch",
        "checkpoint_list",
        "checkpoint_create",
        "checkpoint_restore",
        "afs ws mount",
      ];
    case "read-write":
    default:
      return [
        "file_read",
        "file_write",
        "file_patch",
        "file_grep",
        "afs ws mount",
      ];
  }
}

function buildCreatedSnippet(created: CreatedKey): string {
  return buildMCPClientConfig({
    token: created.token.token ?? "",
    workspaceName: created.workspaceName,
    controlPlane: created.scope === "control-plane",
  });
}

function buildMCPClientConfig({
  token,
  workspaceName,
  controlPlane,
}: {
  token: string;
  workspaceName: string;
  controlPlane: boolean;
}) {
  const serverName = controlPlane
    ? "agent-filesystem"
    : `afs-${(workspaceName || "workspace").trim()}`;
  return JSON.stringify(
    {
      mcpServers: {
        [serverName]: {
          url: `${getControlPlaneURL().replace(/\/+$/, "")}/mcp`,
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      },
    },
    null,
    2,
  );
}

function buildCLILoginSnippet(token: string, workspaceName: string) {
  return [
    `afs auth login --url ${shellQuote(getControlPlaneURL())} --access-token ${shellQuote(token)}`,
    `afs ws mount ${shellQuote(workspaceName || "<workspace>")} <directory>`,
  ].join("\n");
}

function shellQuote(value: string) {
  return `'${value.replaceAll("'", "'\\''")}'`;
}

const CreatedPanel = styled.div`
  display: grid;
  gap: 16px;
`;

const FieldBlock = styled.div`
  display: grid;
  gap: 8px;
`;

const FieldGroup = styled.div`
  display: grid;
  gap: 10px;
`;

const FieldLabel = styled.span`
  color: var(--afs-ink-soft);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
`;

const CodeBlock = styled.pre`
  margin: 0;
  padding: 14px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  overflow: auto;
  max-height: 260px;
`;

const InlineActionsRight = styled.div`
  display: flex;
  justify-content: flex-end;
`;

const ToolPreview = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
`;

const ToolChip = styled.span`
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--afs-line);
  background: var(--afs-panel);
  color: var(--afs-ink-soft);
  font-size: 11px;
  font-weight: 700;
`;

const OptionRow = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;

  @media (max-width: 640px) {
    grid-template-columns: 1fr;
  }
`;

const OptionTile = styled.label<{ $selected: boolean }>`
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid
    ${({ $selected }) =>
      $selected ? "var(--afs-selection-border)" : "var(--afs-line)"};
  background: ${({ $selected }) =>
    $selected ? "var(--afs-selection-bg)" : "var(--afs-panel)"};
  color: ${({ $selected }) =>
    $selected ? "var(--afs-selection-text)" : "var(--afs-ink)"};
  cursor: pointer;
  transition:
    border-color 120ms ease,
    background 120ms ease,
    color 120ms ease;

  &:hover {
    border-color: var(--afs-selection-border);
    background: ${({ $selected }) =>
      $selected ? "var(--afs-selection-bg)" : "var(--afs-selection-hover-bg)"};
    color: ${({ $selected }) =>
      $selected ? "var(--afs-selection-text)" : "var(--afs-selection-hover-ink)"};
  }

  input[type="radio"] {
    margin-top: 3px;
  }
`;

const OptionLabel = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const OptionName = styled.div`
  color: var(--afs-ink);
  font-size: 13.5px;
  font-weight: 700;
`;

const OptionHint = styled.div`
  color: var(--afs-muted);
  font-size: 12px;
  line-height: 1.45;
`;

const MountList = styled.div`
  display: grid;
  gap: 8px;
  border: 1px solid var(--afs-line);
  border-radius: 12px;
  padding: 10px 12px;
  background: var(--afs-panel);
`;

const MountRow = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  padding: 6px 4px;
  border-radius: 8px;

  & + & {
    border-top: 1px dashed var(--afs-line);
    padding-top: 12px;
  }
`;

const MountInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
`;

const MountPath = styled.div`
  color: var(--afs-ink);
  font-family: var(--afs-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 13px;
  font-weight: 600;
`;

const MountVolume = styled.div`
  color: var(--afs-muted);
  font-size: 11.5px;
`;

const MountSelect = styled.div`
  min-width: 220px;

  > * {
    width: 100%;
  }
`;

const MountHint = styled.div`
  color: var(--afs-muted);
  font-size: 12px;
  line-height: 1.5;
`;

const ControlPlaneNotice = styled.div`
  border: 1px solid color-mix(in srgb, #d97706 40%, var(--afs-line));
  background: color-mix(in srgb, #d97706 10%, transparent);
  border-radius: 12px;
  padding: 12px 14px;

  label {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    color: var(--afs-ink);
    font-size: 13px;
    line-height: 1.5;
    cursor: pointer;
  }

  input[type="checkbox"] {
    margin-top: 4px;
  }
`;
