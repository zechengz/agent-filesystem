import { Button, Select } from "@redis-ui/components";
import { useNavigate } from "@tanstack/react-router";
import { Plus } from "lucide-react";
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
} from "../../../components/afs-kit";
import { formatBytes } from "../../../foundation/api/afs";
import {
  useDatabaseScope,
  useScopedWorkspaceSummaries,
} from "../../../foundation/database-scope";
import {
  useCreateWorkspaceCompositionMutation,
  useWorkspaceCompositions,
} from "../../../foundation/hooks/use-afs";
import { WorkspaceCompositionTable } from "../../../foundation/tables/workspace-composition-table";
import type { AFSWorkspaceCompositionSummary } from "../../../foundation/types/afs";
import type { MountMode, WorkspaceOption } from "./types";

export function AgentProfilesTab() {
  const navigate = useNavigate();
  const workspacesQuery = useWorkspaceCompositions();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);

  function openWorkspace(workspace: AFSWorkspaceCompositionSummary) {
    void navigate({
      to: "/workspaces/$workspaceId",
      params: { workspaceId: workspace.id },
    });
  }

  return (
    <>
      <WorkspaceCompositionTable
        rows={workspacesQuery.data ?? []}
        loading={workspacesQuery.isLoading}
        error={workspacesQuery.isError}
        onOpenWorkspace={openWorkspace}
        toolbarAction={
          <Button size="medium" onClick={() => setCreateDialogOpen(true)}>
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            &nbsp;Add Agent Workspace
          </Button>
        }
      />
      <CreateAgentWorkspaceDialog
        open={createDialogOpen}
        onClose={() => setCreateDialogOpen(false)}
      />
    </>
  );
}

type CreateDialogProps = {
  open: boolean;
  onClose: () => void;
};

function mountPathForVolume(name: string) {
  return "/" + name.trim().replace(/^\/+/, "").replace(/\s+/g, "-");
}

function CreateAgentWorkspaceDialog({ open, onClose }: CreateDialogProps) {
  const { databases } = useDatabaseScope();
  const volumesQuery = useScopedWorkspaceSummaries();
  const createWorkspace = useCreateWorkspaceCompositionMutation();

  const volumeOptions = useMemo<WorkspaceOption[]>(
    () =>
      volumesQuery.data.map((volume) => ({
        id: volume.id,
        name: volume.name,
        files: volume.fileCount,
        size: volume.totalBytes === 0 ? "0 KB" : formatBytes(volume.totalBytes),
      })),
    [volumesQuery.data],
  );

  const eligibleDatabases = useMemo(
    () => databases.filter((database) => database.canCreateWorkspaces),
    [databases],
  );
  const defaultDatabase = eligibleDatabases.find((database) => database.isDefault);
  const defaultDatabaseId =
    defaultDatabase != null
      ? defaultDatabase.id
      : eligibleDatabases.length > 0
        ? eligibleDatabases[0].id
        : "";

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [databaseId, setDatabaseId] = useState("");
  const [selectedVolumeIds, setSelectedVolumeIds] = useState<string[]>([]);
  const [mountMode, setMountMode] = useState<MountMode>("r");
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setName("");
    setDescription("");
    setSelectedVolumeIds([]);
    setMountMode("r");
    setFormError(null);
    setDatabaseId(defaultDatabaseId);
  }, [defaultDatabaseId, open]);

  if (!open) return null;

  const busy = createWorkspace.isPending;

  function closeDialog() {
    if (busy) return;
    onClose();
  }

  function toggleVolume(volumeId: string) {
    setSelectedVolumeIds((current) =>
      current.includes(volumeId)
        ? current.filter((id) => id !== volumeId)
        : [...current, volumeId],
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;

    const trimmedName = name.trim();
    if (trimmedName === "") {
      setFormError("Agent Workspace name is required.");
      return;
    }

    setFormError(null);
    try {
      await createWorkspace.mutateAsync({
        name: trimmedName,
        description: description.trim() || undefined,
        databaseId: databaseId || undefined,
        mounts: selectedVolumeIds.flatMap((volumeId) => {
          const volume = volumeOptions.find((item) => item.id === volumeId);
          if (volume == null) return [];
          return [
            {
              volumeId: volume.id,
              volumeName: volume.name,
              mountPath: mountPathForVolume(volume.name),
              readonly: mountMode === "r",
            },
          ];
        }),
      });
      onClose();
    } catch (error) {
      setFormError(
        error instanceof Error
          ? error.message
          : "Unable to create the Agent Workspace.",
      );
    }
  }

  const submitLabel =
    selectedVolumeIds.length === 0
      ? "Create Agent Workspace"
      : selectedVolumeIds.length === 1
        ? "Create with 1 volume"
        : `Create with ${selectedVolumeIds.length} volumes`;

  return (
    <DialogOverlay
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-agent-workspace-title"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) closeDialog();
      }}
    >
      <DialogCard>
        <DialogHeader>
          <div>
            <DialogTitle id="create-agent-workspace-title">
              Add Agent Workspace
            </DialogTitle>
            <DialogBody>
              Name the workspace, then optionally mount existing volumes at
              creation.
            </DialogBody>
          </div>
          <DialogCloseButton
            type="button"
            aria-label="Close"
            onClick={closeDialog}
          >
            &times;
          </DialogCloseButton>
        </DialogHeader>

        <FormGrid onSubmit={submit}>
          <Field>
            Name
            <TextInput
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="coding-agent"
            />
          </Field>

          <Field>
            Description
            <TextInput
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="What this agent can read and write. (optional)"
            />
          </Field>

          {eligibleDatabases.length > 1 ? (
            <Field>
              Database
              <Select
                options={eligibleDatabases.map((database) => ({
                  value: database.id,
                  label: `${database.displayName || database.databaseName}${database.isDefault ? " (default)" : ""}`,
                }))}
                value={databaseId}
                onChange={(next) => setDatabaseId(next)}
              />
            </Field>
          ) : null}

          <Field>
            Initial volumes
            {volumeOptions.length === 0 ? (
              <EmptyVolumes>
                No volumes are available yet. Create this Agent Workspace now and
                add volumes later.
              </EmptyVolumes>
            ) : (
              <VolumeList>
                {volumeOptions.map((volume) => {
                  const selected = selectedVolumeIds.includes(volume.id);
                  return (
                    <VolumeOption key={volume.id} $selected={selected}>
                      <input
                        type="checkbox"
                        checked={selected}
                        aria-label={`Select ${volume.name}`}
                        onChange={() => toggleVolume(volume.id)}
                      />
                      <VolumeOptionMain>
                        <VolumeName>/{volume.name}</VolumeName>
                        <VolumeMeta>
                          {volume.name} &middot; {volume.id}
                        </VolumeMeta>
                      </VolumeOptionMain>
                      <VolumeStats>
                        {volume.files.toLocaleString()} files &middot; {volume.size}
                      </VolumeStats>
                    </VolumeOption>
                  );
                })}
              </VolumeList>
            )}
          </Field>

          <Field>
            Permissions
            <Select
              options={[
                { value: "r", label: "Read only" },
                { value: "rw", label: "Read / write" },
              ]}
              value={mountMode}
              onChange={(value) => setMountMode(value as MountMode)}
            />
          </Field>

          {formError ? <DialogError role="alert">{formError}</DialogError> : null}

          <DialogActions style={{ justifyContent: "flex-end" }}>
            <Button
              size="medium"
              type="button"
              variant="secondary-fill"
              onClick={closeDialog}
              disabled={busy}
            >
              Cancel
            </Button>
            <Button size="medium" type="submit" disabled={busy}>
              {busy ? "Creating..." : submitLabel}
            </Button>
          </DialogActions>
        </FormGrid>
      </DialogCard>
    </DialogOverlay>
  );
}

const VolumeList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 300px;
  overflow: auto;
  padding-right: 4px;
`;

const VolumeOption = styled.label<{ $selected: boolean }>`
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border: 1px solid
    ${({ $selected }) =>
      $selected ? "var(--afs-selection-border)" : "var(--afs-line)"};
  border-radius: 8px;
  background: ${({ $selected }) =>
    $selected ? "var(--afs-selection-bg)" : "var(--afs-panel)"};
  color: ${({ $selected }) => ($selected ? "var(--afs-selection-text)" : "var(--afs-ink)")};
  cursor: pointer;
  transition: background 140ms ease, border-color 140ms ease, color 140ms ease;

  &:hover {
    border-color: var(--afs-selection-border);
    background: ${({ $selected }) => ($selected ? "var(--afs-selection-bg)" : "var(--afs-selection-hover-bg)")};
    color: ${({ $selected }) => ($selected ? "var(--afs-selection-text)" : "var(--afs-selection-hover-ink)")};
  }

  input {
    accent-color: var(--afs-accent);
  }

  @media (max-width: 560px) {
    grid-template-columns: auto minmax(0, 1fr);
  }
`;

const VolumeOptionMain = styled.span`
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
`;

const VolumeName = styled.span`
  color: var(--afs-ink);
  font-family: var(--afs-mono, monospace);
  font-size: 13.5px;
  font-weight: 700;
`;

const VolumeMeta = styled.span`
  min-width: 0;
  color: var(--afs-muted);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const VolumeStats = styled.span`
  color: var(--afs-muted);
  font-size: 12px;
  white-space: nowrap;

  @media (max-width: 560px) {
    grid-column: 2;
    white-space: normal;
  }
`;

const EmptyVolumes = styled.div`
  padding: 18px 14px;
  border: 1px solid var(--afs-line);
  border-radius: 8px;
  color: var(--afs-muted);
  background: var(--afs-panel);
  font-size: 13px;
`;
