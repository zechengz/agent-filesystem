import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { PageStack } from "../components/afs-kit";
import { AgentProfilesTab } from "../features/agents/profiles/AgentProfilesTab";

export const Route = createFileRoute("/workspaces")({
  component: WorkspacesPage,
});

function WorkspacesPage() {
  const location = useLocation();

  if (location.pathname !== "/workspaces") {
    return <Outlet />;
  }

  return (
    <PageStack>
      <AgentProfilesTab />
    </PageStack>
  );
}
