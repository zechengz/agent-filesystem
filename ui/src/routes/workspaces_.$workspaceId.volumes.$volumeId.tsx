import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  VolumeStudio,
  volumeStudioSearchSchema,
} from "../features/volumes/VolumeStudio";
import {
  useWorkspaceComposition,
  workspaceCompositionQueryOptions,
  workspaceQueryOptions,
} from "../foundation/hooks/use-afs";
import { queryClient } from "../foundation/query-client";
import { displayWorkspaceName } from "../foundation/workspace-display";
import type { StudioTab } from "../foundation/workspace-tabs";

export const Route = createFileRoute(
  "/workspaces_/$workspaceId/volumes/$volumeId",
)({
  validateSearch: volumeStudioSearchSchema,
  loader: async ({ params, search }) => {
    await Promise.all([
      queryClient.ensureQueryData({
        ...workspaceCompositionQueryOptions(params.workspaceId),
        revalidateIfStale: true,
      }),
      queryClient.ensureQueryData({
        ...workspaceQueryOptions(search?.databaseId ?? null, params.volumeId),
        revalidateIfStale: true,
      }),
    ]);
  },
  component: WorkspaceVolumeStudioRoute,
});

function WorkspaceVolumeStudioRoute() {
  const navigate = useNavigate();
  const { workspaceId, volumeId } = Route.useParams();
  const search = Route.useSearch();
  const compositionQuery = useWorkspaceComposition(workspaceId);
  const workspaceLabel = displayWorkspaceName(
    compositionQuery.data?.name ?? workspaceId,
  );

  function selfSearch(tab: StudioTab) {
    return {
      ...(search.databaseId ? { databaseId: search.databaseId } : {}),
      ...(tab === "browse" ? {} : { tab }),
    };
  }

  return (
    <VolumeStudio
      volumeId={volumeId}
      databaseId={search.databaseId ?? null}
      activeTab={search.tab}
      welcome={search.welcome}
      breadcrumbItems={[
        {
          label: "Agent Workspaces",
          onClick: () => {
            void navigate({ to: "/workspaces" });
          },
        },
        {
          label: workspaceLabel,
          onClick: () => {
            void navigate({
              to: "/workspaces/$workspaceId",
              params: { workspaceId },
            });
          },
        },
      ]}
      onNavigateToSelf={({ tab, replace }) => {
        void navigate({
          to: "/workspaces/$workspaceId/volumes/$volumeId",
          params: { workspaceId, volumeId },
          search: selfSearch(tab),
          replace,
        });
      }}
      onDeleted={() => {
        void navigate({
          to: "/workspaces/$workspaceId",
          params: { workspaceId },
          replace: true,
        });
      }}
    />
  );
}
