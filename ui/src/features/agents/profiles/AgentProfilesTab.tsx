import { Button, Typography } from "@redis-ui/components";
import { Table } from "@redis-ui/table";
import type { ColumnDef } from "@redis-ui/table";
import { useMemo, useState } from "react";
import styled from "styled-components";
import { Tag } from "../../../components/afs-kit";
import * as S from "../../../foundation/tables/workspace-table.styles";
import { AgentProfileDrawer } from "./AgentProfileDrawer";
import {
  initialAgents,
  profilePalette,
  sampleFilesystems,
  sampleTokens,
} from "./sample-data";
import type { AgentProfile } from "./types";

type ViewMode = "table" | "cards";

function newDraftId(): string {
  return "new_" + Math.random().toString(36).slice(2, 7);
}

function newAgentId(): string {
  return "agt_" + Math.random().toString(36).slice(2, 8);
}

export function AgentProfilesTab() {
  const [agents, setAgents] = useState<AgentProfile[]>(initialAgents);
  const [openId, setOpenId] = useState<string | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>("table");
  const [search, setSearch] = useState("");

  const visibleAgents = useMemo(
    () => agents.filter((a) => !a.draft),
    [agents],
  );

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return visibleAgents;
    return visibleAgents.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.id.toLowerCase().includes(q) ||
        a.description.toLowerCase().includes(q),
    );
  }, [visibleAgents, search]);

  const openAgent = useMemo(() => {
    if (!openId) return null;
    return agents.find((a) => a.id === openId) ?? null;
  }, [openId, agents]);

  function startCreate() {
    const id = newDraftId();
    const draft: AgentProfile = {
      id,
      name: "",
      description: "",
      letter: "A",
      color: profilePalette[agents.length % profilePalette.length],
      status: "idle",
      sharedCount: 0,
      perRunCount: 0,
      tokens: 0,
      lastActive: "never",
      draft: true,
    };
    setAgents((prev) => [draft, ...prev]);
    sampleFilesystems[id] = { shared: [], perRun: [] };
    sampleTokens[id] = [];
    setOpenId(id);
    setIsNew(true);
  }

  function saveAgent(updated: AgentProfile) {
    if (isNew) {
      const realId = newAgentId();
      setAgents((prev) =>
        prev.map((a) =>
          a.id === updated.id
            ? { ...updated, id: realId, draft: false }
            : a,
        ),
      );
      sampleFilesystems[realId] = sampleFilesystems[updated.id] ?? {
        shared: [],
        perRun: [],
      };
      sampleTokens[realId] = sampleTokens[updated.id] ?? [];
      setIsNew(false);
      setOpenId(null);
    } else {
      setAgents((prev) =>
        prev.map((a) => (a.id === updated.id ? { ...a, ...updated } : a)),
      );
      setOpenId(null);
    }
  }

  function closeDrawer() {
    if (isNew && openAgent && openAgent.draft) {
      setAgents((prev) => prev.filter((a) => a.id !== openAgent.id));
    }
    setOpenId(null);
    setIsNew(false);
  }

  function deleteAgent(id: string) {
    setAgents((prev) => prev.filter((a) => a.id !== id));
    setOpenId(null);
    setIsNew(false);
  }

  const columns = useMemo(
    () =>
      [
        {
          accessorKey: "name",
          header: "Profile",
          size: 280,
          enableSorting: false,
          cell: ({ row }) => {
            const a = row.original;
            return (
              <NameCell>
                <Avatar style={{ background: a.color }}>{a.letter}</Avatar>
                <NameText>
                  <strong>{a.name || "Unnamed profile"}</strong>
                  <span title={a.description || a.id}>
                    {a.description || a.id}
                  </span>
                </NameText>
              </NameCell>
            );
          },
        },
        {
          accessorKey: "filesystem",
          header: "Filesystem",
          size: 160,
          enableSorting: false,
          cell: ({ row }) => {
            const a = row.original;
            const total = a.sharedCount + a.perRunCount;
            return (
              <Typography.Body component="span">
                {total} {total === 1 ? "workspace" : "workspaces"}
              </Typography.Body>
            );
          },
        },
        {
          accessorKey: "tokens",
          header: "Tokens",
          size: 130,
          enableSorting: false,
          cell: ({ row }) => {
            const a = row.original;
            return a.tokens > 0 ? (
              <Typography.Body component="span">
                {a.tokens} active
              </Typography.Body>
            ) : (
              <Typography.Body component="span" color="secondary">
                no tokens
              </Typography.Body>
            );
          },
        },
        {
          accessorKey: "status",
          header: "Status",
          size: 130,
          enableSorting: false,
          cell: ({ row }) => {
            const a = row.original;
            return (
              <StatusPill>
                <StatusDot $live={a.status === "live"} />
                {a.status === "live" ? "Connected" : "Idle"}
              </StatusPill>
            );
          },
        },
        {
          accessorKey: "lastActive",
          header: "Last active",
          size: 140,
          enableSorting: false,
          cell: ({ row }) => (
            <Typography.Body component="span" color="secondary">
              {row.original.lastActive}
            </Typography.Body>
          ),
        },
      ] as ColumnDef<AgentProfile>[],
    [],
  );

  const isFiltering = search.trim().length > 0;

  return (
    <>
      <S.TableBlock>
        <S.HeadingWrap style={{ padding: 0 }}>
          <S.SearchInput
            value={search}
            onChange={setSearch}
            placeholder="Search profiles by name, ID, or description..."
          />
          <S.ToggleGroup>
            <S.ToggleButton
              $active={viewMode === "cards"}
              aria-pressed={viewMode === "cards"}
              onClick={() => setViewMode("cards")}
            >
              Cards
            </S.ToggleButton>
            <S.ToggleButton
              $active={viewMode === "table"}
              aria-pressed={viewMode === "table"}
              onClick={() => setViewMode("table")}
            >
              Table
            </S.ToggleButton>
          </S.ToggleGroup>
          <Button size="medium" onClick={startCreate}>
            New agent profile
          </Button>
        </S.HeadingWrap>

        {visibleAgents.length === 0 ? (
          <S.EmptyState>
            <Typography.Heading
              component="h3"
              size="XS"
              style={{ margin: 0 }}
            >
              No agent profiles yet
            </Typography.Heading>
            <Typography.Body
              color="secondary"
              component="p"
              style={{ margin: "8px auto 14px", maxWidth: 460 }}
            >
              An agent profile says &ldquo;this is the coding agent, here are
              the files it can see.&rdquo; Set one up to give an agent a
              filesystem and a token to connect with.
            </Typography.Body>
            <Button size="medium" onClick={startCreate}>
              New agent profile
            </Button>
          </S.EmptyState>
        ) : filtered.length === 0 ? (
          <S.EmptyState>
            {isFiltering
              ? "No profiles match the current filter."
              : "No profiles yet."}
          </S.EmptyState>
        ) : viewMode === "table" ? (
          <S.TableCard>
            <S.DenseTableViewport>
              <Table
                columns={columns}
                data={filtered}
                getRowId={(row) => row.id}
                onRowClick={(row) => {
                  setOpenId(row.id);
                  setIsNew(false);
                }}
                stripedRows
              />
            </S.DenseTableViewport>
          </S.TableCard>
        ) : (
          <CardGrid>
            {filtered.map((a) => (
              <ProfileCard
                key={a.id}
                onClick={() => {
                  setOpenId(a.id);
                  setIsNew(false);
                }}
              >
                <CardHead>
                  <Avatar style={{ background: a.color }}>{a.letter}</Avatar>
                  <CardHeadText>
                    <strong>{a.name || "Unnamed profile"}</strong>
                    <CardId>{a.id}</CardId>
                  </CardHeadText>
                  <StatusPill>
                    <StatusDot $live={a.status === "live"} />
                    {a.status === "live" ? "Connected" : "Idle"}
                  </StatusPill>
                </CardHead>
                <CardDescription>
                  {a.description ||
                    "No description yet. Open this profile to add one."}
                </CardDescription>
                <CardTags>
                  <Tag>{a.sharedCount} shared</Tag>
                  <Tag>{a.perRunCount} per-run</Tag>
                  <Tag>
                    {a.tokens > 0
                      ? `${a.tokens} ${a.tokens === 1 ? "token" : "tokens"}`
                      : "no tokens"}
                  </Tag>
                </CardTags>
                <CardFoot>last active &middot; {a.lastActive}</CardFoot>
              </ProfileCard>
            ))}
          </CardGrid>
        )}
      </S.TableBlock>

      {openAgent != null ? (
        <AgentProfileDrawer
          agent={openAgent}
          isNew={isNew}
          onClose={closeDrawer}
          onSave={saveAgent}
          onDelete={deleteAgent}
        />
      ) : null}
    </>
  );
}

const NameCell = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
`;

const Avatar = styled.div`
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 13px;
`;

const NameText = styled.div`
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;

  strong {
    color: var(--afs-ink);
    font-size: 14px;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  span {
    color: var(--afs-muted);
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
`;

const StatusPill = styled.span`
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 999px;
  border: 1px solid var(--afs-line);
  background: var(--afs-panel);
  color: var(--afs-ink-soft);
  font-size: 12px;
  font-weight: 600;
`;

const StatusDot = styled.span<{ $live: boolean }>`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${({ $live }) => ($live ? "#10b981" : "#a1a1aa")};
  ${({ $live }) =>
    $live ? "box-shadow: 0 0 0 3px rgba(16,185,129,0.16);" : ""}
`;

const CardGrid = styled.div`
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fill, minmax(max(28%, 320px), 1fr));
`;

const ProfileCard = styled.button`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px;
  border-radius: 14px;
  border: 1px solid var(--afs-line);
  background: var(--afs-panel-strong);
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;

  &:hover {
    border-color: var(--afs-line-strong);
    box-shadow: 0 8px 24px rgba(8, 6, 13, 0.08);
    transform: translateY(-2px);
  }
`;

const CardHead = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
`;

const CardHeadText = styled.div`
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;

  strong {
    color: var(--afs-ink);
    font-size: 14.5px;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
`;

const CardId = styled.span`
  color: var(--afs-muted);
  font-size: 12px;
  font-family: var(--afs-mono, monospace);
`;

const CardDescription = styled.p`
  margin: 0;
  color: var(--afs-muted);
  font-size: 13px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
`;

const CardTags = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
`;

const CardFoot = styled.span`
  color: var(--afs-muted);
  font-size: 12px;
`;
