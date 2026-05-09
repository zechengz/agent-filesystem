// AgentProfileDrawer — slide-over for editing an agent profile.
// Layout (header / tabs / body / footer) lives here; the shell + animation
// are provided by the shared <Drawer> component.

import { Button } from "@redis-ui/components";
import { useState } from "react";
import styled from "styled-components";
import { TabButton, Tabs } from "../../../components/afs-kit";
import { Drawer } from "../../../components/drawer";
import { CreateTokenDialog } from "./CreateTokenDialog";
import { MountsSection } from "./MountsSection";
import { SettingsSection } from "./SettingsSection";
import { TokensSection } from "./TokensSection";
import {
  availableWorkspaces,
  sampleFilesystems,
  sampleTokens,
} from "./sample-data";
import type {
  AgentProfile,
  AgentToken,
  CreateTokenInput,
  Filesystem,
  MountMode,
  MountPersistence,
} from "./types";

type DrawerTabKey = "filesystem" | "tokens" | "settings";

export type AgentProfileDrawerProps = {
  agent: AgentProfile;
  isNew: boolean;
  onClose: () => void;
  onSave: (next: AgentProfile) => void;
  onDelete: (id: string) => void;
};

function nextRandomId(prefix: string): string {
  return prefix + Math.random().toString(36).slice(2, 8);
}

function makeSecret(): string {
  return "afs_live_" + Math.random().toString(36).slice(2, 18);
}

export function AgentProfileDrawer({
  agent,
  isNew,
  onClose,
  onSave,
  onDelete,
}: AgentProfileDrawerProps) {
  const [tab, setTab] = useState<DrawerTabKey>("filesystem");
  const [name, setName] = useState(agent.name || "");
  const [description, setDescription] = useState(agent.description || "");
  const [fs, setFs] = useState<Filesystem>(() =>
    sampleFilesystems[agent.id]
      ? { ...sampleFilesystems[agent.id] }
      : { shared: [], perRun: [] },
  );
  const [tokens, setTokens] = useState<AgentToken[]>(() =>
    sampleTokens[agent.id] ? [...sampleTokens[agent.id]] : [],
  );
  const [tokenDialogOpen, setTokenDialogOpen] = useState(false);
  const [newToken, setNewToken] = useState<AgentToken | null>(null);

  function addRow(kind: MountPersistence) {
    setFs((prev) => {
      const usedIds = new Set(
        [...prev.shared, ...prev.perRun].map((m) => m.wsId),
      );
      const next = availableWorkspaces.find((w) => !usedIds.has(w.id));
      if (!next) return prev;
      const row = {
        wsId: next.id,
        mount: "/" + next.name,
        mode: "r" as MountMode,
      };
      return { ...prev, [kind]: [...prev[kind], row] };
    });
  }

  function updateRow(
    kind: MountPersistence,
    idx: number,
    patch: { wsId?: string; mount?: string; mode?: MountMode },
  ) {
    setFs((prev) => ({
      ...prev,
      [kind]: prev[kind].map((r, i) => (i === idx ? { ...r, ...patch } : r)),
    }));
  }

  function moveRow(
    fromKind: MountPersistence,
    fromIdx: number,
    toKind: MountPersistence,
  ) {
    setFs((prev) => {
      const row = prev[fromKind][fromIdx];
      if (!row) return prev;
      return {
        ...prev,
        [fromKind]: prev[fromKind].filter((_, i) => i !== fromIdx),
        [toKind]: [...prev[toKind], { ...row }],
      };
    });
  }

  function removeRow(kind: MountPersistence, idx: number) {
    setFs((prev) => ({
      ...prev,
      [kind]: prev[kind].filter((_, i) => i !== idx),
    }));
  }

  function saveAgent() {
    if (!name.trim()) return;
    const next: AgentProfile = {
      ...agent,
      name,
      description,
      sharedCount: fs.shared.length,
      perRunCount: fs.perRun.length,
      tokens: tokens.length,
      letter: (name[0] || agent.letter || "A").toUpperCase(),
    };
    sampleFilesystems[agent.id] = { ...fs };
    sampleTokens[agent.id] = [...tokens];
    onSave(next);
  }

  function issueToken(input: CreateTokenInput) {
    const created: AgentToken = {
      id: nextRandomId("t_"),
      name: input.name || "Untitled token",
      secret: makeSecret(),
      created: "just now",
      lastUsed: "never",
      scopes: input.scopes,
    };
    setTokens((prev) => [created, ...prev]);
    setNewToken(created);
    setTokenDialogOpen(false);
  }

  function revokeToken(id: string) {
    setTokens((prev) => prev.filter((t) => t.id !== id));
  }

  return (
    <>
      <Drawer
        onClose={onClose}
        ariaLabel={isNew ? "New agent profile" : `Edit ${agent.name}`}
        width="min(880px, 96vw)"
      >
        {({ requestClose }) => (
          <>
            <Header>
              <Identity>
                <Avatar style={{ background: agent.color }}>
                  {agent.letter || (name[0] || "A").toUpperCase()}
                </Avatar>
                <IdentityFields>
                  <NameInput
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder={
                      isNew
                        ? 'Name this profile… (e.g. "Coding Agent")'
                        : agent.name
                    }
                  />
                  <DescriptionInput
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    placeholder='What this agent is for — e.g. "reads our repo, writes patches to scratch"'
                  />
                  <IdLine>
                    <span>{isNew ? "ID assigned on save" : agent.id}</span>
                    {!isNew ? (
                      <>
                        <span aria-hidden>·</span>
                        <StatusLine>
                          <StatusDot $live={agent.status === "live"} />
                          {agent.status === "live" ? "connected" : "idle"}
                        </StatusLine>
                      </>
                    ) : null}
                  </IdLine>
                </IdentityFields>
              </Identity>
              <CloseButton
                type="button"
                onClick={requestClose}
                aria-label="Close"
              >
                ×
              </CloseButton>
            </Header>

            <TabsRow>
              <Tabs>
                <TabButton
                  $active={tab === "filesystem"}
                  onClick={() => setTab("filesystem")}
                >
                  Filesystem
                </TabButton>
                <TabButton
                  $active={tab === "tokens"}
                  onClick={() => setTab("tokens")}
                >
                  Tokens · {tokens.length}
                </TabButton>
                <TabButton
                  $active={tab === "settings"}
                  onClick={() => setTab("settings")}
                >
                  Settings
                </TabButton>
              </Tabs>
            </TabsRow>

            <Body>
              {tab === "filesystem" ? (
                <MountsSection
                  fs={fs}
                  onAddRow={addRow}
                  onUpdateRow={updateRow}
                  onMoveRow={moveRow}
                  onRemoveRow={removeRow}
                />
              ) : null}
              {tab === "tokens" ? (
                <TokensSection
                  tokens={tokens}
                  newToken={newToken}
                  onIssueToken={() => setTokenDialogOpen(true)}
                  onRevokeToken={revokeToken}
                  onClearNew={() => setNewToken(null)}
                />
              ) : null}
              {tab === "settings" ? (
                <SettingsSection
                  agent={agent}
                  name={name}
                  description={description}
                  isNew={isNew}
                  onChangeName={setName}
                  onChangeDescription={setDescription}
                  onDelete={onDelete}
                />
              ) : null}
            </Body>

            <Footer>
              <FooterLeft>
                {!isNew ? (
                  <Button
                    size="medium"
                    variant="secondary-ghost"
                    onClick={() => onDelete(agent.id)}
                  >
                    Delete profile
                  </Button>
                ) : (
                  <FooterHint>
                    You can edit the filesystem any time after saving.
                  </FooterHint>
                )}
              </FooterLeft>
              <FooterRight>
                <Button
                  size="medium"
                  variant="secondary-fill"
                  onClick={requestClose}
                >
                  Cancel
                </Button>
                <Button
                  size="medium"
                  onClick={saveAgent}
                  disabled={!name.trim()}
                >
                  {isNew ? "Save agent profile" : "Save changes"}
                </Button>
              </FooterRight>
            </Footer>
          </>
        )}
      </Drawer>

      <CreateTokenDialog
        isOpen={tokenDialogOpen}
        onClose={() => setTokenDialogOpen(false)}
        onCreate={issueToken}
      />
    </>
  );
}

const Header = styled.div`
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--afs-line);
`;

const Identity = styled.div`
  display: flex;
  align-items: flex-start;
  gap: 14px;
  flex: 1;
  min-width: 0;
`;

const Avatar = styled.div`
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 16px;
`;

const IdentityFields = styled.div`
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const NameInput = styled.input`
  border: 0;
  outline: 0;
  background: transparent;
  font-size: 18px;
  font-weight: 700;
  color: var(--afs-ink);
  padding: 4px 6px;
  margin-left: -6px;
  border-radius: 8px;
  width: 100%;

  &:focus {
    background: var(--afs-bg-soft);
  }

  &::placeholder {
    color: var(--afs-muted);
  }
`;

const DescriptionInput = styled(NameInput)`
  font-size: 13px;
  font-weight: 400;
  color: var(--afs-muted);
`;

const IdLine = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 4px;
  font-family: var(--afs-mono, monospace);
  font-size: 12px;
  color: var(--afs-muted);
`;

const StatusLine = styled.span`
  display: inline-flex;
  align-items: center;
  gap: 6px;
`;

const StatusDot = styled.span<{ $live: boolean }>`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${({ $live }) => ($live ? "#10b981" : "#a1a1aa")};
  ${({ $live }) =>
    $live ? "box-shadow: 0 0 0 3px rgba(16,185,129,0.18);" : ""}
`;

const CloseButton = styled.button`
  flex: 0 0 auto;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--afs-line);
  background: transparent;
  color: var(--afs-muted);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  transition: background 120ms ease, color 120ms ease;

  &:hover {
    background: var(--afs-bg-soft);
    color: var(--afs-ink);
  }
`;

const TabsRow = styled.div`
  padding: 12px 22px 0;
  border-bottom: 1px solid var(--afs-line);
`;

const Body = styled.div`
  flex: 1;
  overflow-y: auto;
  padding: 18px 22px 28px;
`;

const Footer = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 22px;
  border-top: 1px solid var(--afs-line);
  background: var(--afs-panel);
`;

const FooterLeft = styled.div`
  display: flex;
  align-items: center;
  min-width: 0;
`;

const FooterRight = styled.div`
  display: flex;
  gap: 10px;
  align-items: center;
`;

const FooterHint = styled.span`
  color: var(--afs-muted);
  font-size: 13px;
`;
