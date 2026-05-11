import { Button } from "@redis-ui/components";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import styled from "styled-components";
import {
  Field,
  FieldGroup,
  DialogError,
  NoticeBody,
  NoticeCard,
  NoticeTitle,
  PageStack,
  TabButton,
  Tabs,
  TextArea,
  TextInput,
  TwoColumnFields,
} from "../../../components/afs-kit";
import { SurfaceCard } from "../../../components/card-shell";
import { formatBytes } from "../../../foundation/api/afs";
import { useScopedWorkspaceSummaries } from "../../../foundation/database-scope";
import {
  useCreateWorkspaceCompositionMutation,
  useDeleteWorkspaceCompositionMutation,
  useReplaceWorkspaceCompositionMountsMutation,
  useUpdateWorkspaceCompositionMutation,
  useWorkspaceComposition,
} from "../../../foundation/hooks/use-afs";
import type {
  AFSWorkspaceCompositionDetail,
  AFSWorkspaceCompositionMount,
} from "../../../foundation/types/afs";
import { MountsSection } from "./MountsSection";
import { WorkspaceTokensSection } from "./WorkspaceTokensSection";
import type { Filesystem, Mount, MountMode, WorkspaceOption } from "./types";

type ProfileTabKey = "filesystem" | "tokens" | "settings";

type Props = {
  profileId: string;
};

function normalizeMountPath(path: string) {
  const trimmed = path.trim();
  if (trimmed === "" || trimmed === "/") return "/";
  return "/" + trimmed.replace(/^\/+/, "").replace(/\/+$/, "");
}

function compositionToFilesystem(
  composition: AFSWorkspaceCompositionDetail | null | undefined,
): Filesystem {
  if (composition == null) return emptyFilesystem();
  return {
    shared: composition.mounts.map((mount) => ({
      wsId: mount.volumeId,
      mount: mount.mountPath,
      mode: mount.readonly ? "r" : "rw",
    })),
    perRun: [],
  };
}

function filesystemToMounts(fs: Filesystem): AFSWorkspaceCompositionMount[] {
  return [...fs.shared, ...fs.perRun].map((mount) => ({
    volumeId: mount.wsId,
    mountPath: normalizeMountPath(mount.mount),
    readonly: mount.mode !== "rw",
  }));
}

function emptyFilesystem(): Filesystem {
  return { shared: [], perRun: [] };
}

function updateMountRow(
  fs: Filesystem,
  kind: "shared" | "perRun",
  idx: number,
  patch: { wsId?: string; mount?: string; mode?: MountMode },
): Filesystem {
  return {
    ...fs,
    [kind]: fs[kind].map((row, rowIndex) =>
      rowIndex === idx ? { ...row, ...patch } : row,
    ),
  };
}

export function AgentProfilePage({ profileId }: Props) {
  const navigate = useNavigate();
  const isNew = profileId === "new";
  const compositionQuery = useWorkspaceComposition(profileId, !isNew);
  const volumesQuery = useScopedWorkspaceSummaries();
  const createWorkspace = useCreateWorkspaceCompositionMutation();
  const updateWorkspace = useUpdateWorkspaceCompositionMutation();
  const replaceMounts = useReplaceWorkspaceCompositionMountsMutation();
  const deleteWorkspace = useDeleteWorkspaceCompositionMutation();

  const [tab, setTab] = useState<ProfileTabKey>("filesystem");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [savedName, setSavedName] = useState("");
  const [savedDescription, setSavedDescription] = useState("");
  const [fs, setFs] = useState<Filesystem>(emptyFilesystem);
  const [loadedWorkspaceId, setLoadedWorkspaceId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const composition = compositionQuery.data ?? null;

  useEffect(() => {
    if (isNew) {
      if (loadedWorkspaceId !== "new") {
        setName("");
        setDescription("");
        setSavedName("");
        setSavedDescription("");
        setFs(emptyFilesystem());
        setLoadedWorkspaceId("new");
      }
      return;
    }
    if (composition == null || loadedWorkspaceId === composition.id) return;
    setName(composition.name);
    setDescription(composition.description ?? "");
    setSavedName(composition.name);
    setSavedDescription(composition.description ?? "");
    setFs(compositionToFilesystem(composition));
    setLoadedWorkspaceId(composition.id);
  }, [composition, isNew, loadedWorkspaceId]);

  const volumeOptions = useMemo<WorkspaceOption[]>(() => {
    const merged = new Map<string, WorkspaceOption>();
    for (const volume of volumesQuery.data) {
      merged.set(volume.id, {
        id: volume.id,
        name: volume.name,
        files: volume.fileCount,
        size: volume.totalBytes === 0 ? "0 KB" : formatBytes(volume.totalBytes),
      });
    }
    for (const mount of composition?.mounts ?? []) {
      if (!merged.has(mount.volumeId)) {
        merged.set(mount.volumeId, {
          id: mount.volumeId,
          name: mount.volumeName ?? mount.volumeId,
          files: 0,
          size: "0 KB",
        });
      }
    }
    return Array.from(merged.values());
  }, [composition?.mounts, volumesQuery.data]);

  const pending =
    createWorkspace.isPending ||
    updateWorkspace.isPending ||
    replaceMounts.isPending ||
    deleteWorkspace.isPending;
  const identityDirty =
    name.trim() !== savedName || description.trim() !== savedDescription;

  function closePage() {
    void navigate({ to: "/workspaces" });
  }

  function renderBreadcrumb(label: string) {
    return (
      <StudioNavRow>
        <BreadcrumbGroup>
          <BreadcrumbButton type="button" onClick={closePage}>
            <BackArrow aria-hidden>&#8592;</BackArrow>
            Back to Agent Workspaces
          </BreadcrumbButton>
          <BreadcrumbSeparator>/</BreadcrumbSeparator>
          <BreadcrumbCurrent>{label}</BreadcrumbCurrent>
        </BreadcrumbGroup>
      </StudioNavRow>
    );
  }

  function addVolumes(mounts: Mount[]) {
    const usedIds = new Set(fs.shared.map((mount) => mount.wsId));
    const nextMounts: Mount[] = [];
    for (const mount of mounts) {
      if (usedIds.has(mount.wsId)) continue;
      usedIds.add(mount.wsId);
      nextMounts.push(mount);
    }
    if (nextMounts.length === 0) return;
    const nextFs = {
      shared: [...fs.shared, ...nextMounts],
      perRun: [],
    };
    setFs(nextFs);
    void saveMounts(nextFs);
  }

  async function saveWorkspaceIdentity() {
    if (!name.trim() || pending) return;
    setFormError(null);
    try {
      if (isNew) {
        const created = await createWorkspace.mutateAsync({
          name: name.trim(),
          description: description.trim() || undefined,
          mounts: filesystemToMounts(fs),
        });
        setSavedName(created.name);
        setSavedDescription(created.description ?? "");
        void navigate({
          to: "/workspaces/$workspaceId",
          params: { workspaceId: created.id },
        });
        return;
      }
      const updated = await updateWorkspace.mutateAsync({
        workspaceId: profileId,
        name: name.trim(),
        description: description.trim() || undefined,
      });
      setSavedName(updated.name);
      setSavedDescription(updated.description ?? "");
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to save workspace.",
      );
    }
  }

  async function saveMounts(nextFs: Filesystem) {
    if (isNew) return;
    setFormError(null);
    try {
      await replaceMounts.mutateAsync({
        workspaceId: profileId,
        mounts: filesystemToMounts(nextFs),
      });
    } catch (error) {
      setFormError(
        error instanceof Error
          ? error.message
          : "Unable to save mounted volumes.",
      );
    }
  }

  async function deleteCurrentWorkspace() {
    if (isNew || pending) return;
    setFormError(null);
    try {
      await deleteWorkspace.mutateAsync(profileId);
      closePage();
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to delete workspace.",
      );
    }
  }

  if (!isNew && compositionQuery.isLoading) {
    return (
      <PageStack>
        {renderBreadcrumb("Loading...")}
        <NoticeCard>
          <NoticeTitle>Loading Agent Workspace</NoticeTitle>
          <NoticeBody>Fetching the Agent Workspace manifest.</NoticeBody>
        </NoticeCard>
      </PageStack>
    );
  }

  if (!isNew && (compositionQuery.isError || composition == null)) {
    return (
      <PageStack>
        {renderBreadcrumb("Not found")}
        <NoticeCard $tone="warning" role="alert">
          <NoticeTitle>Agent Workspace not found</NoticeTitle>
          <NoticeBody>
            This Agent Workspace may have been deleted or moved.
          </NoticeBody>
        </NoticeCard>
      </PageStack>
    );
  }

  const breadcrumbLabel =
    name.trim() || composition?.name || (isNew ? "New Agent Workspace" : profileId);

  return (
    <ProfilePageStack>
      {renderBreadcrumb(breadcrumbLabel)}

      <EditorCard>
        <DetailName>{breadcrumbLabel}</DetailName>

        <Tabs>
          <TabButton
            $active={tab === "filesystem"}
            onClick={() => setTab("filesystem")}
          >
            Filesystem
          </TabButton>
          <TabButton $active={tab === "tokens"} onClick={() => setTab("tokens")}>
            Tokens
          </TabButton>
          <TabButton
            $active={tab === "settings"}
            onClick={() => setTab("settings")}
          >
            Settings
          </TabButton>
        </Tabs>

        <BodyRegion>
          {tab === "filesystem" ? (
            <MountsSection
              fs={fs}
              volumes={volumeOptions}
              onAddVolumes={addVolumes}
              onUpdateRow={(kind, idx, patch) =>
                {
                  const nextFs = updateMountRow(fs, kind, idx, patch);
                  setFs(nextFs);
                  void saveMounts(nextFs);
                }
              }
              onRemoveRow={(kind, idx) =>
                {
                  const nextFs = {
                    ...fs,
                    [kind]: fs[kind].filter((_, index) => index !== idx),
                  };
                  setFs(nextFs);
                  void saveMounts(nextFs);
                }
              }
            />
          ) : null}
          {tab === "tokens" ? (
            <WorkspaceTokensSection
              workspaceId={profileId}
              workspaceName={breadcrumbLabel}
              disabled={isNew}
            />
          ) : null}
          {tab === "settings" ? (
            <SettingsPanel>
              <FieldGroup title="Identity">
                <TwoColumnFields>
                  <Field>
                    Display name
                    <TextInput
                      value={name}
                      onChange={(event) => setName(event.target.value)}
                      placeholder='e.g. "Coding Agent"'
                    />
                  </Field>
                  <Field>
                    Workspace ID
                    <TextInput
                      value={isNew ? "assigned on save" : profileId}
                      readOnly
                    />
                  </Field>
                </TwoColumnFields>
                <Field>
                  Description
                  <TextArea
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    placeholder="What this Agent Workspace is for"
                  />
                </Field>
                {formError ? (
                  <DialogError role="alert">{formError}</DialogError>
                ) : null}
                <SettingsActions>
                  <Button
                    size="medium"
                    onClick={() => void saveWorkspaceIdentity()}
                    disabled={!name.trim() || !identityDirty || pending}
                  >
                    {createWorkspace.isPending || updateWorkspace.isPending
                      ? "Saving..."
                      : isNew
                        ? "Save Agent Workspace"
                        : "Save changes"}
                  </Button>
                </SettingsActions>
              </FieldGroup>

              {!isNew ? (
                <DangerZoneCard>
                  <DangerZoneHeader>
                    <DangerZoneTitle>Delete Agent Workspace</DangerZoneTitle>
                    <DangerZoneDesc>
                      Permanently remove <strong>{name || "this workspace"}</strong>{" "}
                      from the Agent Workspace registry. Mounted volumes are not
                      deleted.
                    </DangerZoneDesc>
                  </DangerZoneHeader>
                  <DangerZoneActions>
                    <DeleteWorkspaceButton
                      size="large"
                      disabled={pending}
                      onClick={() => void deleteCurrentWorkspace()}
                    >
                      {deleteWorkspace.isPending
                        ? "Deleting..."
                        : "Delete Agent Workspace"}
                    </DeleteWorkspaceButton>
                  </DangerZoneActions>
                </DangerZoneCard>
              ) : null}
            </SettingsPanel>
          ) : null}
        </BodyRegion>
      </EditorCard>
    </ProfilePageStack>
  );
}

const ProfilePageStack = styled(PageStack)`
  padding-bottom: 112px;
`;

const StudioNavRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 24px;

  @media (max-width: 1040px) {
    align-items: flex-start;
    flex-wrap: wrap;
  }
`;

const BreadcrumbGroup = styled.div`
  display: flex;
  align-items: center;
  gap: 10px;
`;

const BreadcrumbButton = styled.button`
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  padding: 0;
  color: var(--afs-muted);
  font: inherit;
  font-size: 14px;
  font-weight: 400;
  cursor: pointer;

  &:hover {
    color: var(--afs-ink);
    text-decoration: underline;
  }
`;

const BackArrow = styled.span`
  font-size: 16px;
  line-height: 1;
`;

const BreadcrumbSeparator = styled.span`
  color: var(--afs-muted);
  font-size: 14px;
`;

const BreadcrumbCurrent = styled.span`
  color: var(--afs-ink);
  font-size: 14px;
  font-weight: 700;
`;

const DetailName = styled.h1`
  margin: 0;
  color: var(--afs-ink);
  font-size: 28px;
  font-weight: 700;
  line-height: 1.18;
  letter-spacing: 0;
`;

const EditorCard = styled(SurfaceCard)`
  display: flex;
  flex-direction: column;
  gap: 24px;
  min-height: calc(100vh - 210px);
  padding: 22px;
  overflow: hidden;

  @media (max-width: 720px) {
    padding: 16px;
  }
`;

const BodyRegion = styled.div`
  flex: 1 1 auto;
  min-height: 0;
`;

const SettingsPanel = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const SettingsActions = styled.div`
  display: flex;
  justify-content: flex-end;
`;

const DangerZoneCard = styled(SurfaceCard)`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 24px;
  border: 1px solid rgba(220, 38, 38, 0.2);
  background: rgba(220, 38, 38, 0.03);

  @media (max-width: 720px) {
    flex-direction: column;
    align-items: flex-start;
  }
`;

const DangerZoneHeader = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const DangerZoneActions = styled.div`
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 12px;

  @media (max-width: 720px) {
    width: 100%;
    align-items: flex-start;
  }
`;

const DangerZoneTitle = styled.h3`
  margin: 0;
  color: #dc2626;
  font-size: 15px;
  font-weight: 700;
`;

const DangerZoneDesc = styled.p`
  margin: 0;
  color: var(--afs-muted);
  font-size: 13px;
  line-height: 1.5;
`;

const DeleteWorkspaceButton = styled(Button)`
  && {
    white-space: nowrap;
    background: ${({ theme }) => theme.semantic.color.background.danger500};
    color: ${({ theme }) => theme.semantic.color.text.inverse};
    box-shadow: none;
  }

  &&:hover:not(:disabled),
  &&:focus-visible:not(:disabled) {
    background: ${({ theme }) => theme.semantic.color.background.danger600};
    color: ${({ theme }) => theme.semantic.color.text.inverse};
    box-shadow: none;
  }
`;
