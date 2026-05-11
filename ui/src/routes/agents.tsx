import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader } from "@redis-ui/components";
import { useEffect } from "react";
import { z } from "zod";

const agentsSearchSchema = z.object({
  workspaceId: z.string().optional(),
  databaseId: z.string().optional(),
  tab: z.string().optional(),
});

export const Route = createFileRoute("/agents")({
  validateSearch: agentsSearchSchema,
  component: AgentsRedirectPage,
});

function AgentsRedirectPage() {
  const navigate = useNavigate();

  useEffect(() => {
    void navigate({ to: "/", replace: true });
  }, [navigate]);

  return <Loader data-testid="loader--spinner" />;
}
