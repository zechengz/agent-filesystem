// /volumes — content-tree list view. Volumes are the forkable trees with files,
// checkpoints, search, and activity. Workspaces compose one or more volumes.

import { Button, Loader } from "@redis-ui/components";
import {
  createFileRoute,
  Outlet,
  useLocation,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { useMemo, useState } from "react";
import styled from "styled-components";
import { PageStack } from "../components/afs-kit";
import { CreateWorkspaceDialog } from "../features/workspaces/CreateWorkspaceDialog";
import {
  agentsQueryOptions,
  workspaceSummariesQueryOptions,
} from "../foundation/hooks/use-afs";
import {
  useDatabaseScope,
  useScopedAgents,
  useScopedWorkspaceSummaries,
} from "../foundation/database-scope";
import { queryClient } from "../foundation/query-client";
import { WorkspaceTable } from "../foundation/tables/workspace-table";
import type { AFSWorkspaceSummary } from "../foundation/types/afs";
import type { StudioTab } from "../foundation/workspace-tabs";
import { useDrawerCommands } from "../foundation/drawer-context";
import type { CommandsDrawerConfig } from "../foundation/drawer-context";

const WORKSPACE_COMMANDS: CommandsDrawerConfig = {
  title: "Work with volumes",
  subline: "Common commands. Run from your shell.",
  sections: [
    {
      title: "Create",
      description: "Provision a new volume.",
      command: "afs vol create my-volume",
    },
    {
      title: "Import",
      description: "Import a local directory as a volume.",
      command: "afs vol import my-volume ./docs",
    },
    {
      title: "Fork",
      description: "Branch a copy for an experiment.",
      command: "afs vol fork my-volume my-experiment",
    },
    {
      title: "Delete",
      description: "Tear down a volume you no longer need.",
      command: "afs vol delete my-volume",
    },
  ],
};

export const Route = createFileRoute("/volumes")({
  loader: async () => {
    await Promise.all([
      queryClient.ensureQueryData({
        ...workspaceSummariesQueryOptions(null),
        revalidateIfStale: true,
      }),
      queryClient.ensureQueryData({ ...agentsQueryOptions(null), revalidateIfStale: true }),
    ]);
  },
  component: VolumesPage,
});

function workspaceRowKey(databaseId: string | undefined, workspaceId: string) {
  return `${databaseId ?? ""}:${workspaceId}`;
}

function VolumesPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const router = useRouter();
  const workspacesQuery = useScopedWorkspaceSummaries();
  const agentsQuery = useScopedAgents();
  const { databases } = useDatabaseScope();
  const [createVolumeOpen, setCreateVolumeOpen] = useState(false);
  // Register contextual commands so the global Help button opens this page's
  // volume command reference. Memoized so the registration is stable.
  const drawerConfig = useMemo(() => WORKSPACE_COMMANDS, []);
  useDrawerCommands(
    location.pathname === "/volumes" ? drawerConfig : null,
  );

  if (location.pathname !== "/volumes") {
    return <Outlet />;
  }

  if (workspacesQuery.isLoading) {
    return <Loader data-testid="loader--spinner" />;
  }

  const workspaces = workspacesQuery.data;
  const connectedAgentsByWorkspace = agentsQuery.data.reduce<Record<string, number>>(
    (counts, session) => {
      const key = workspaceRowKey(session.databaseId, session.workspaceId);
      counts[key] = (counts[key] ?? 0) + 1;
      return counts;
    },
    {},
  );
  const databaseSearchSupportById = Object.fromEntries(
    databases.map((database) => [database.id, database.supportsSearch]),
  );

  function openWorkspace(workspace: AFSWorkspaceSummary) {
    void navigate({
      to: "/volumes/$volumeId",
      params: { volumeId: workspace.id },
      search: { databaseId: workspace.databaseId },
    });
  }

  function previewWorkspace(workspace: AFSWorkspaceSummary) {
    void router.preloadRoute({
      to: "/volumes/$volumeId",
      params: { volumeId: workspace.id },
      search: { databaseId: workspace.databaseId },
    }).catch(() => undefined);
  }

  function openWorkspaceTab(
    workspace: AFSWorkspaceSummary,
    tab: StudioTab,
  ) {
    void navigate({
      to: "/volumes/$volumeId",
      params: { volumeId: workspace.id },
      search: {
        databaseId: workspace.databaseId,
        ...(tab === "browse" ? {} : { tab }),
      },
    });
  }

  return (
    <WorkspacesPageStack>
      <WorkspaceTable
        rows={workspaces}
        loading={workspacesQuery.isLoading}
        error={workspacesQuery.isError}
        fillAvailableHeight
        connectedAgentsByWorkspace={connectedAgentsByWorkspace}
        databaseSearchSupportById={databaseSearchSupportById}
        onOpenWorkspace={openWorkspace}
        onPreviewWorkspace={previewWorkspace}
        onOpenWorkspaceTab={openWorkspaceTab}
        toolbarAction={
          <Button size="medium" onClick={() => setCreateVolumeOpen(true)}>
            Add Volume
          </Button>
        }
        resourceLabel="volume"
        resourcePluralLabel="volumes"
        idLabel="volume ID"
        // intentionally no onEditWorkspace / onDeleteWorkspace — managed via CLI.
      />
      <CreateWorkspaceDialog
        open={createVolumeOpen}
        onClose={() => setCreateVolumeOpen(false)}
        resourceLabel="volume"
      />
    </WorkspacesPageStack>
  );
}

const WorkspacesPageStack = styled(PageStack)`
  flex: 1 1 auto;
  min-height: 0;
`;
