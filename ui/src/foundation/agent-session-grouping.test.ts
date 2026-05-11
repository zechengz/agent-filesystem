import { describe, expect, test } from "vitest";
import { groupMountedAgentWorkspaceSessions } from "./agent-session-grouping";
import type {
  AFSAgentSession,
  AFSWorkspaceCompositionSummary,
} from "./types/afs";

const baseAgent: AFSAgentSession = {
  sessionId: "sess",
  workspaceId: "vol_repo",
  workspaceName: "repo",
  databaseId: "db_1",
  databaseName: "local",
  agentId: "agent_1",
  agentName: "Codex",
  sessionName: "coding-agent /repo",
  clientKind: "sync",
  afsVersion: "0.1.0",
  hostname: "devbox",
  operatingSystem: "darwin",
  localPath: "/Users/rowan/coding-agent/repo",
  label: "coding-agent /repo",
  readonly: false,
  state: "active",
  startedAt: "2026-05-10T10:00:00Z",
  lastSeenAt: "2026-05-10T10:02:00Z",
  leaseExpiresAt: "2026-05-10T10:03:00Z",
};

const agentWorkspace: AFSWorkspaceCompositionSummary = {
  id: "aw_coding",
  name: "coding-agent",
  databaseId: "db_1",
  databaseName: "local",
  mountCount: 2,
  mountedVolumes: [
    { id: "vol_repo", name: "repo", mountPath: "/repo", readonly: false },
    { id: "vol_memory", name: "memory", mountPath: "/memory", readonly: false },
  ],
  connectedAgentCount: 2,
  updatedAt: "2026-05-10T10:00:00Z",
};

function agent(overrides: Partial<AFSAgentSession>): AFSAgentSession {
  return { ...baseAgent, ...overrides };
}

describe("groupMountedAgentWorkspaceSessions", () => {
  test("collapses child volume sessions into one mounted Agent Workspace session", () => {
    const grouped = groupMountedAgentWorkspaceSessions(
      [
        agent({ sessionId: "sess_repo" }),
        agent({
          sessionId: "sess_memory",
          workspaceId: "vol_memory",
          workspaceName: "memory",
          sessionName: "coding-agent /memory",
          label: "coding-agent /memory",
          localPath: "/Users/rowan/coding-agent/memory",
          lastSeenAt: "2026-05-10T10:04:00Z",
        }),
      ],
      [agentWorkspace],
    );

    expect(grouped).toHaveLength(1);
    expect(grouped[0]).toMatchObject({
      workspaceId: "aw_coding",
      workspaceName: "coding-agent",
      sessionName: "Codex",
      label: "Codex",
      localPath: "/Users/rowan/coding-agent",
      lastSeenAt: "2026-05-10T10:04:00Z",
    });
  });

  test("keeps direct volume mounts separate from Agent Workspace manifests", () => {
    const grouped = groupMountedAgentWorkspaceSessions(
      [
        agent({
          sessionId: "sess_direct",
          sessionName: "direct repo",
          label: "direct repo",
        }),
      ],
      [agentWorkspace],
    );

    expect(grouped).toHaveLength(1);
    expect(grouped[0]?.workspaceId).toBe("vol_repo");
    expect(grouped[0]?.sessionName).toBe("direct repo");
  });
});
