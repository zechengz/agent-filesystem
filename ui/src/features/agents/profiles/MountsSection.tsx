import { Button, Select } from "@redis-ui/components";
import { Folder, Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import styled from "styled-components";
import {
  Field,
  MetaItem,
  StatGrid,
  StatLabel,
  StatValue,
  StatDetail,
  TextInput,
  Tag,
  ToneChip,
} from "../../../components/afs-kit";
import { availableWorkspaces } from "./sample-data";
import type {
  Filesystem,
  Mount,
  MountMode,
  MountPersistence,
} from "./types";

type Props = {
  fs: Filesystem;
  onAddRow: (kind: MountPersistence) => void;
  onUpdateRow: (
    kind: MountPersistence,
    idx: number,
    patch: { wsId?: string; mount?: string; mode?: MountMode },
  ) => void;
  onMoveRow: (
    fromKind: MountPersistence,
    fromIdx: number,
    toKind: MountPersistence,
  ) => void;
  onRemoveRow: (kind: MountPersistence, idx: number) => void;
};

type RowWithMeta = Mount & { kind: MountPersistence; idx: number };

function bytesFromSize(s: string | undefined): number {
  if (!s) return 0;
  const m = s.match(/([\d.]+)\s*(B|KB|MB|GB)?/i);
  if (!m) return 0;
  const n = parseFloat(m[1]);
  const u = (m[2] || "B").toUpperCase();
  const factors: Record<string, number> = {
    B: 1,
    KB: 1024,
    MB: 1024 ** 2,
    GB: 1024 ** 3,
  };
  return n * (factors[u] || 1);
}

function formatSize(b: number): string {
  if (b < 1024) return `${b} B`;
  if (b < 1024 ** 2)
    return `${(b / 1024).toFixed(b < 10240 ? 1 : 0)} KB`;
  if (b < 1024 ** 3) return `${(b / 1024 ** 2).toFixed(1)} MB`;
  return `${(b / 1024 ** 3).toFixed(2)} GB`;
}

export function MountsSection({
  fs,
  onAddRow,
  onUpdateRow,
  onMoveRow,
  onRemoveRow,
}: Props) {
  const [editing, setEditing] = useState<{
    kind: MountPersistence;
    idx: number;
  } | null>(null);

  const rows: RowWithMeta[] = useMemo(() => {
    const out: RowWithMeta[] = [];
    fs.shared.forEach((r, i) => out.push({ ...r, kind: "shared", idx: i }));
    fs.perRun.forEach((r, i) => out.push({ ...r, kind: "perRun", idx: i }));
    return out;
  }, [fs]);

  const usedIds = new Set([...fs.shared, ...fs.perRun].map((m) => m.wsId));
  function availableFor(currentId: string) {
    return availableWorkspaces.filter(
      (w) => !usedIds.has(w.id) || w.id === currentId,
    );
  }

  const totalFiles = rows.reduce(
    (s, r) =>
      s + (availableWorkspaces.find((w) => w.id === r.wsId)?.files ?? 0),
    0,
  );
  const totalBytes = rows.reduce(
    (s, r) =>
      s +
      bytesFromSize(availableWorkspaces.find((w) => w.id === r.wsId)?.size),
    0,
  );

  return (
    <Stack>
      <StatGrid>
        <MetaItem>
          <StatLabel>Mounted folders</StatLabel>
          <StatValue>{rows.length}</StatValue>
          <StatDetail>
            {fs.shared.length} shared · {fs.perRun.length} per-run
          </StatDetail>
        </MetaItem>
        <MetaItem>
          <StatLabel>Total files</StatLabel>
          <StatValue>{totalFiles.toLocaleString()}</StatValue>
          <StatDetail>across all workspaces</StatDetail>
        </MetaItem>
        <MetaItem>
          <StatLabel>Total size</StatLabel>
          <StatValue>{formatSize(totalBytes)}</StatValue>
          <StatDetail>summed across mounts</StatDetail>
        </MetaItem>
      </StatGrid>

      <RowList>
        <RootRow>
          <Folder size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>
            <strong>/</strong> &mdash; filesystem root
          </span>
          <RootMeta>
            {rows.length} {rows.length === 1 ? "workspace" : "workspaces"} ·{" "}
            {formatSize(totalBytes)}
          </RootMeta>
        </RootRow>

        {rows.length === 0 ? (
          <EmptyHint>
            No folders yet. Add one below to start building this agent's
            filesystem.
          </EmptyHint>
        ) : null}

        {rows.map((row) => {
          const ws = availableWorkspaces.find((w) => w.id === row.wsId);
          const isEditing =
            editing != null &&
            editing.kind === row.kind &&
            editing.idx === row.idx;
          return (
            <MountRow key={`${row.kind}-${row.idx}`} $editing={isEditing}>
              <RowMain>
                <Folder size={18} strokeWidth={1.75} aria-hidden="true" />
                <RowTitle>
                  <RowPath>{row.mount}</RowPath>
                  <RowSub>
                    <strong>{ws?.name}</strong>
                    {" · "}
                    <span>{ws?.id}</span>
                  </RowSub>
                </RowTitle>
              </RowMain>

              <RowFiles>
                <RowFilesValue>
                  {ws?.files.toLocaleString() ?? "—"}
                </RowFilesValue>
                <RowFilesLabel>
                  files · {ws?.size ?? "—"}
                </RowFilesLabel>
              </RowFiles>

              <RowBadges>
                <ToneChip $tone={row.kind === "shared" ? "git-import" : "blank"}>
                  {row.kind === "shared" ? "Shared" : "Per-run"}
                </ToneChip>
                <Tag>{row.mode === "rw" ? "read / write" : "read"}</Tag>
              </RowBadges>

              <RowActions>
                <IconButton
                  type="button"
                  aria-label="Edit mount"
                  onClick={() =>
                    setEditing(
                      isEditing ? null : { kind: row.kind, idx: row.idx },
                    )
                  }
                >
                  <Pencil size={16} strokeWidth={1.75} aria-hidden="true" />
                </IconButton>
                <IconButton
                  type="button"
                  aria-label="Remove mount"
                  onClick={() => {
                    onRemoveRow(row.kind, row.idx);
                    if (isEditing) setEditing(null);
                  }}
                >
                  <Trash2 size={16} strokeWidth={1.75} aria-hidden="true" />
                </IconButton>
              </RowActions>

              {isEditing ? (
                <EditPanel>
                  <Field>
                    Workspace
                    <Select
                      options={availableFor(row.wsId).map((w) => ({
                        value: w.id,
                        label: w.name,
                      }))}
                      value={row.wsId}
                      onChange={(value) => {
                        const next = availableWorkspaces.find(
                          (w) => w.id === value,
                        );
                        onUpdateRow(row.kind, row.idx, {
                          wsId: value,
                          mount: "/" + (next?.name ?? "mount"),
                        });
                      }}
                    />
                  </Field>
                  <Field>
                    Mount path
                    <TextInput
                      value={row.mount}
                      onChange={(event) =>
                        onUpdateRow(row.kind, row.idx, {
                          mount: event.target.value,
                        })
                      }
                    />
                  </Field>
                  <Field>
                    Persistence
                    <Select
                      options={[
                        { value: "shared", label: "Shared" },
                        { value: "perRun", label: "Per-run" },
                      ]}
                      value={row.kind}
                      onChange={(value) => {
                        const next = value as MountPersistence;
                        if (next !== row.kind) {
                          onMoveRow(row.kind, row.idx, next);
                          setEditing(null);
                        }
                      }}
                    />
                  </Field>
                  <Field>
                    Permissions
                    <Select
                      options={[
                        { value: "r", label: "Read only" },
                        { value: "rw", label: "Read / write" },
                      ]}
                      value={row.mode}
                      onChange={(value) =>
                        onUpdateRow(row.kind, row.idx, {
                          mode: value as MountMode,
                        })
                      }
                    />
                  </Field>
                </EditPanel>
              ) : null}
            </MountRow>
          );
        })}
      </RowList>

      <AddRow>
        <Button
          size="small"
          variant="secondary-fill"
          onClick={() => onAddRow("shared")}
        >
          <Plus size={14} strokeWidth={2} aria-hidden="true" />
          &nbsp;Add shared folder
        </Button>
        <Button
          size="small"
          variant="secondary-fill"
          onClick={() => onAddRow("perRun")}
        >
          <Plus size={14} strokeWidth={2} aria-hidden="true" />
          &nbsp;Add per-run folder
        </Button>
      </AddRow>

      <ExplainerGrid>
        <Explainer>
          <ExplainerTitle>Shared workspace</ExplainerTitle>
          <ExplainerBody>
            One folder, persistent across all runs. Every run reads and writes
            the same files. Use for memory, knowledge bases, sources.
          </ExplainerBody>
        </Explainer>
        <Explainer>
          <ExplainerTitle>Per-run workspace</ExplainerTitle>
          <ExplainerBody>
            A fresh copy is mounted at the start of each run and saved or
            discarded when the run ends. Use for scratch space and sandboxes
            that must not leak between sessions.
          </ExplainerBody>
        </Explainer>
      </ExplainerGrid>
    </Stack>
  );
}

const Stack = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const RowList = styled.div`
  border: 1px solid var(--afs-line);
  border-radius: 12px;
  overflow: hidden;
  background: var(--afs-panel);
`;

const RootRow = styled.div`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--afs-line);
  color: var(--afs-muted);
  font-size: 13px;

  strong {
    color: var(--afs-ink);
  }
`;

const RootMeta = styled.span`
  margin-left: auto;
  font-size: 12px;
  color: var(--afs-muted);
`;

const EmptyHint = styled.div`
  padding: 28px 16px;
  text-align: center;
  color: var(--afs-muted);
  font-size: 13px;
`;

const MountRow = styled.div<{ $editing: boolean }>`
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto auto;
  gap: 16px;
  align-items: center;
  padding: 12px 14px;
  border-bottom: 1px solid var(--afs-line);
  background: ${({ $editing }) =>
    $editing ? "var(--afs-bg-soft)" : "transparent"};

  &:last-child {
    border-bottom: 0;
  }

  @media (max-width: 760px) {
    grid-template-columns: minmax(0, 1fr) auto;
    row-gap: 8px;
  }
`;

const RowMain = styled.div`
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--afs-muted);
`;

const RowTitle = styled.div`
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const RowPath = styled.span`
  color: var(--afs-ink);
  font-weight: 700;
  font-family: var(--afs-mono, monospace);
  font-size: 13.5px;
`;

const RowSub = styled.span`
  color: var(--afs-muted);
  font-size: 12px;

  strong {
    color: var(--afs-ink-soft);
    font-weight: 600;
  }
`;

const RowFiles = styled.div`
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  color: var(--afs-ink-soft);

  @media (max-width: 760px) {
    align-items: flex-start;
  }
`;

const RowFilesValue = styled.span`
  font-family: var(--afs-mono, monospace);
  font-size: 13px;
`;

const RowFilesLabel = styled.span`
  color: var(--afs-muted);
  font-size: 11px;
`;

const RowBadges = styled.div`
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
`;

const RowActions = styled.div`
  display: flex;
  gap: 4px;

  @media (max-width: 760px) {
    grid-column: 1 / -1;
    justify-content: flex-end;
  }
`;

const IconButton = styled.button`
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--afs-line);
  background: var(--afs-panel);
  color: var(--afs-ink-soft);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 140ms ease, border-color 140ms ease;

  &:hover {
    background: var(--afs-bg-soft);
    border-color: var(--afs-line-strong);
  }
`;

const EditPanel = styled.div`
  grid-column: 1 / -1;
  margin-top: 6px;
  padding: 14px;
  border-top: 1px solid var(--afs-line);
  background: var(--afs-bg-soft);
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;

  @media (max-width: 760px) {
    grid-template-columns: 1fr;
  }
`;

const AddRow = styled.div`
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
`;

const ExplainerGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;

  @media (max-width: 720px) {
    grid-template-columns: 1fr;
  }
`;

const Explainer = styled.div`
  border: 1px solid var(--afs-line);
  border-radius: 12px;
  padding: 14px 16px;
  background: var(--afs-panel);
`;

const ExplainerTitle = styled.div`
  color: var(--afs-ink);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-bottom: 6px;
`;

const ExplainerBody = styled.p`
  margin: 0;
  color: var(--afs-muted);
  font-size: 13px;
  line-height: 1.5;
`;
