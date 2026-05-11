import type {
  AFSAgentSession,
  AFSWorkspaceCompositionSummary,
  AFSWorkspaceCompositionVolumeLabel,
} from "./types/afs";

type WorkspaceMountMatch = {
  workspace: AFSWorkspaceCompositionSummary;
  volume: AFSWorkspaceCompositionVolumeLabel;
};

type PendingAgentGroup = {
  firstIndex: number;
  workspace: AFSWorkspaceCompositionSummary;
  localRoot: string;
  agents: AFSAgentSession[];
};

type AgentGroupEntry =
  | {
      kind: "single";
      firstIndex: number;
      agent: AFSAgentSession;
    }
  | {
      kind: "workspace";
      group: PendingAgentGroup;
    };

const ACTIVE_STATE_ORDER = ["active", "syncing", "starting"];

export function groupMountedAgentWorkspaceSessions(
  agents: AFSAgentSession[],
  agentWorkspaces: AFSWorkspaceCompositionSummary[] = [],
): AFSAgentSession[] {
  if (agents.length === 0 || agentWorkspaces.length === 0) {
    return agents;
  }

  const entries: AgentGroupEntry[] = [];
  const groups = new Map<string, PendingAgentGroup>();

  agents.forEach((agent, index) => {
    const match = findAgentWorkspaceMount(agent, agentWorkspaces);
    if (match == null) {
      entries.push({ kind: "single", firstIndex: index, agent });
      return;
    }

    const localRoot = workspaceMountLocalRoot(
      agent.localPath,
      match.volume.mountPath,
    );
    const groupKey = [
      match.workspace.id,
      agent.hostname.trim(),
      localRoot,
    ].join("\u0000");
    let group = groups.get(groupKey);
    if (group == null) {
      group = {
        firstIndex: index,
        workspace: match.workspace,
        localRoot,
        agents: [],
      };
      groups.set(groupKey, group);
      entries.push({ kind: "workspace", group });
    }
    group.agents.push(agent);
  });

  return entries
    .sort((left, right) => entryIndex(left) - entryIndex(right))
    .map((entry) =>
      entry.kind === "single"
        ? entry.agent
        : groupedWorkspaceAgentSession(entry.group),
    );
}

function entryIndex(entry: AgentGroupEntry) {
  return entry.kind === "single" ? entry.firstIndex : entry.group.firstIndex;
}

function findAgentWorkspaceMount(
  agent: AFSAgentSession,
  agentWorkspaces: AFSWorkspaceCompositionSummary[],
): WorkspaceMountMatch | null {
  const volumeId = agent.workspaceId.trim();
  if (volumeId === "") {
    return null;
  }

  for (const workspace of agentWorkspaces) {
    for (const volume of workspace.mountedVolumes) {
      if (volume.id !== volumeId) {
        continue;
      }
      if (agentMatchesCompositionMount(agent, workspace, volume)) {
        return { workspace, volume };
      }
    }
  }
  return null;
}

function agentMatchesCompositionMount(
  agent: AFSAgentSession,
  workspace: AFSWorkspaceCompositionSummary,
  volume: AFSWorkspaceCompositionVolumeLabel,
) {
  const expected = workspaceSessionName(
    workspace.name,
    volume.mountPath,
  );
  return [agent.sessionName, agent.label]
    .map((value) => value?.trim() ?? "")
    .some((value) => value === expected);
}

function workspaceSessionName(workspaceName: string, mountPath: string) {
  const name = workspaceName.trim();
  const path = normalizeCompositionMountPath(mountPath);
  return name === "" ? path : `${name} ${path}`;
}

function normalizeCompositionMountPath(mountPath: string) {
  const trimmed = mountPath.trim();
  if (trimmed === "" || trimmed === "/") {
    return "/";
  }
  return `/${trimmed.replace(/^\/+/, "").replace(/\/+$/, "")}`;
}

function workspaceMountLocalRoot(localPath: string, mountPath: string) {
  const normalizedLocal = localPath.trim().replace(/\/+$/, "");
  if (normalizedLocal === "") {
    return "";
  }

  const normalizedMount = normalizeCompositionMountPath(mountPath);
  if (normalizedMount === "/") {
    return normalizedLocal;
  }

  const suffix = normalizedMount.replace(/^\/+/, "");
  const localSuffix = `/${suffix}`;
  if (!normalizedLocal.endsWith(localSuffix)) {
    return normalizedLocal;
  }

  const root = normalizedLocal.slice(0, -localSuffix.length);
  return root || normalizedLocal;
}

function groupedWorkspaceAgentSession(group: PendingAgentGroup): AFSAgentSession {
  const primary = latestAgent(group.agents);
  const clientKinds = uniqueNonEmpty(group.agents.map((agent) => agent.clientKind));
  const agentIdentity = groupedAgentIdentity(group.agents);

  return {
    ...primary,
    sessionId: workspaceAgentSessionId(group),
    workspaceId: group.workspace.id,
    workspaceName: group.workspace.name,
    databaseId: group.workspace.databaseId ?? primary.databaseId,
    databaseName: group.workspace.databaseName ?? primary.databaseName,
    sessionName: agentIdentity,
    localPath: group.localRoot || primary.localPath,
    label: agentIdentity || group.workspace.name,
    clientKind:
      clientKinds.length <= 1
        ? clientKinds[0] ?? primary.clientKind
        : `${clientKinds.length} clients`,
    readonly: group.agents.every((agent) => agent.readonly),
    state: groupedState(group.agents),
    startedAt: boundaryDate(group.agents, "startedAt", "min"),
    lastSeenAt: boundaryDate(group.agents, "lastSeenAt", "max"),
    leaseExpiresAt: boundaryDate(group.agents, "leaseExpiresAt", "max"),
  };
}

function groupedAgentIdentity(agents: AFSAgentSession[]) {
  const agentNames = uniqueNonEmpty(agents.map((agent) => agent.agentName ?? ""));
  if (agentNames.length === 1) {
    return agentNames[0];
  }
  if (agentNames.length > 1) {
    return `${agentNames.length} agents`;
  }

  const agentIds = uniqueNonEmpty(agents.map((agent) => agent.agentId ?? ""));
  if (agentIds.length === 1) {
    return agentIds[0];
  }
  if (agentIds.length > 1) {
    return `${agentIds.length} agents`;
  }

  return undefined;
}

function workspaceAgentSessionId(group: PendingAgentGroup) {
  return [
    "agent-workspace",
    group.workspace.id,
    group.agents[0]?.hostname.trim() ?? "",
    group.localRoot,
  ].join(":");
}

function latestAgent(agents: AFSAgentSession[]) {
  return [...agents].sort(
    (left, right) => Date.parse(right.lastSeenAt) - Date.parse(left.lastSeenAt),
  )[0] ?? agents[0];
}

function uniqueNonEmpty(values: string[]) {
  return Array.from(
    new Set(values.map((value) => value.trim()).filter((value) => value !== "")),
  );
}

function groupedState(agents: AFSAgentSession[]) {
  for (const state of ACTIVE_STATE_ORDER) {
    if (agents.some((agent) => agent.state === state)) {
      return state;
    }
  }
  return latestAgent(agents).state;
}

function boundaryDate(
  agents: AFSAgentSession[],
  field: "startedAt" | "lastSeenAt" | "leaseExpiresAt",
  direction: "min" | "max",
) {
  const sorted = [...agents].sort((left, right) => {
    const delta = Date.parse(left[field]) - Date.parse(right[field]);
    return direction === "min" ? delta : delta * -1;
  });
  return sorted[0]?.[field] ?? latestAgent(agents)[field];
}
