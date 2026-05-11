import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import { APIKeysPage } from "../features/api-keys/APIKeysPage";

const searchSchema = z.object({
  workspaceId: z.string().optional(),
  databaseId: z.string().optional(),
});

export const Route = createFileRoute("/api-keys")({
  validateSearch: searchSchema,
  component: APIKeysRouteComponent,
});

function APIKeysRouteComponent() {
  const search = Route.useSearch();
  return <APIKeysPage search={search} basePath="/api-keys" />;
}
