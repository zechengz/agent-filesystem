import { Button, Typography } from "@redis-ui/components";
import { Check, Copy, Trash2 } from "lucide-react";
import { useState } from "react";
import styled from "styled-components";
import {
  EmptyState,
  NoticeBody,
  NoticeCard,
  NoticeTitle,
  Tag,
} from "../../../components/afs-kit";
import type { AgentToken } from "./types";

type Props = {
  tokens: AgentToken[];
  newToken: AgentToken | null;
  onIssueToken: () => void;
  onRevokeToken: (id: string) => void;
  onClearNew: () => void;
};

export function TokensSection({
  tokens,
  newToken,
  onIssueToken,
  onRevokeToken,
  onClearNew,
}: Props) {
  const [copied, setCopied] = useState<string | null>(null);

  function copy(value: string, label: string) {
    void navigator.clipboard?.writeText(value);
    setCopied(label);
    window.setTimeout(() => setCopied(null), 2000);
  }

  return (
    <Stack>
      <Header>
        <div>
          <Typography.Heading component="h3" size="XS" style={{ margin: 0 }}>
            API tokens
          </Typography.Heading>
          <Typography.Body color="secondary" component="p" style={{ margin: "4px 0 0" }}>
            Tokens authenticate this agent with the AFS server. Treat them like
            passwords.
          </Typography.Body>
        </div>
        <Button size="medium" onClick={onIssueToken}>
          Generate token
        </Button>
      </Header>

      {newToken ? (
        <NoticeCard $tone="warning">
          <NoticeTitle>Save this token now &mdash; you won't see it again</NoticeTitle>
          <NoticeBody>
            <SecretRow>
              <SecretCode>{newToken.secret}</SecretCode>
              <SecretActions>
                <Button
                  size="small"
                  variant="secondary-fill"
                  onClick={() => copy(newToken.secret, "secret")}
                >
                  {copied === "secret" ? (
                    <>
                      <Check size={14} strokeWidth={2} aria-hidden="true" />
                      &nbsp;Copied
                    </>
                  ) : (
                    <>
                      <Copy size={14} strokeWidth={2} aria-hidden="true" />
                      &nbsp;Copy
                    </>
                  )}
                </Button>
                <Button
                  size="small"
                  variant="secondary-fill"
                  onClick={onClearNew}
                >
                  Dismiss
                </Button>
              </SecretActions>
            </SecretRow>
          </NoticeBody>
        </NoticeCard>
      ) : null}

      {tokens.length === 0 ? (
        <EmptyState>
          <Typography.Heading
            component="h4"
            size="XS"
            style={{ margin: 0 }}
          >
            No tokens yet
          </Typography.Heading>
          <Typography.Body
            color="secondary"
            component="p"
            style={{ margin: "6px 0 12px" }}
          >
            Generate a token so this agent can authenticate. You'll see the
            secret once &mdash; copy it then.
          </Typography.Body>
          <Button size="medium" onClick={onIssueToken}>
            Generate token
          </Button>
        </EmptyState>
      ) : (
        <TokenList>
          {tokens.map((token) => (
            <TokenItem key={token.id}>
              <TokenInfo>
                <TokenName>{token.name}</TokenName>
                <TokenScopes>
                  {token.scopes.map((scope) => (
                    <Tag key={scope}>{scope}</Tag>
                  ))}
                </TokenScopes>
              </TokenInfo>
              <SecretCode title={token.secret}>{token.secret}</SecretCode>
              <TokenMeta>
                <span>Created {token.created}</span>
                <span>Last used {token.lastUsed}</span>
              </TokenMeta>
              <TokenActions>
                <Button
                  size="small"
                  variant="secondary-fill"
                  onClick={() => copy(token.secret, token.id)}
                >
                  {copied === token.id ? (
                    <>
                      <Check size={14} strokeWidth={2} aria-hidden="true" />
                      &nbsp;Copied
                    </>
                  ) : (
                    <>
                      <Copy size={14} strokeWidth={2} aria-hidden="true" />
                      &nbsp;Copy
                    </>
                  )}
                </Button>
                <Button
                  size="small"
                  variant="secondary-ghost"
                  onClick={() => onRevokeToken(token.id)}
                  aria-label="Revoke token"
                >
                  <Trash2 size={14} strokeWidth={1.75} aria-hidden="true" />
                  &nbsp;Revoke
                </Button>
              </TokenActions>
            </TokenItem>
          ))}
        </TokenList>
      )}
    </Stack>
  );
}

const Stack = styled.div`
  display: flex;
  flex-direction: column;
  gap: 14px;
`;

const Header = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
`;

const SecretRow = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 8px;
`;

const SecretCode = styled.code`
  display: inline-block;
  padding: 4px 8px;
  border-radius: 8px;
  border: 1px solid var(--afs-line);
  background: var(--afs-panel);
  color: var(--afs-ink);
  font-family: var(--afs-mono, monospace);
  font-size: 12px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const SecretActions = styled.div`
  display: flex;
  gap: 8px;
`;

const TokenList = styled.div`
  border: 1px solid var(--afs-line);
  border-radius: 12px;
  overflow: hidden;
  background: var(--afs-panel);
`;

const TokenItem = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1.4fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--afs-line);

  &:last-child {
    border-bottom: 0;
  }

  @media (max-width: 820px) {
    grid-template-columns: 1fr;
  }
`;

const TokenInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
`;

const TokenName = styled.span`
  color: var(--afs-ink);
  font-size: 14px;
  font-weight: 700;
`;

const TokenScopes = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
`;

const TokenMeta = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: var(--afs-muted);
  font-size: 12px;
`;

const TokenActions = styled.div`
  display: flex;
  gap: 8px;
  justify-content: flex-end;

  @media (max-width: 820px) {
    justify-content: flex-start;
  }
`;
