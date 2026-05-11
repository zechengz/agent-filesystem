import { createFileRoute } from "@tanstack/react-router";
import { AgentProfilePage } from "../features/agents/profiles/AgentProfilePage";

export const Route = createFileRoute("/workspaces/$workspaceId")({
  component: AgentProfileRoute,
});

function AgentProfileRoute() {
  const { workspaceId } = Route.useParams();
  return <AgentProfilePage profileId={workspaceId} />;
}
