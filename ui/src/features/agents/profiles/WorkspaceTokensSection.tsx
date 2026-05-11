import { Button, Select, Typography } from "@redis-ui/components";
import { useState } from "react";
import type { FormEvent } from "react";
import styled from "styled-components";
import {
  DialogError,
  Field,
  FormGrid,
  NoticeBody,
  NoticeCard,
  NoticeTitle,
  TextInput,
  TwoColumnFields,
} from "../../../components/afs-kit";
import { getControlPlaneURL } from "../../../foundation/api/afs";
import { useCreateCLIAccessTokenMutation } from "../../../foundation/hooks/use-afs";
import type {
  AFSCLIAccessToken,
  AFSCLIAccessTokenCapability,
} from "../../../foundation/types/afs";

type Props = {
  workspaceId: string;
  workspaceName: string;
  disabled?: boolean;
};

type ExpiryOption = "24h" | "7d" | "30d" | "never";

export function WorkspaceTokensSection({
  workspaceId,
  workspaceName,
  disabled,
}: Props) {
  const createToken = useCreateCLIAccessTokenMutation();
  const [name, setName] = useState("");
  const [permission, setPermission] =
    useState<AFSCLIAccessTokenCapability>("mount-rw");
  const [expiry, setExpiry] = useState<ExpiryOption>("30d");
  const [createdToken, setCreatedToken] = useState<AFSCLIAccessToken | null>(
    null,
  );
  const [copied, setCopied] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const pending = createToken.isPending;
  const tokenValue = createdToken?.token ?? "";
  const targetWorkspace = createdToken?.workspaceName ?? workspaceName;
  const loginCommand =
    tokenValue === ""
      ? ""
      : `afs auth login --url ${shellQuote(getControlPlaneURL())} --access-token ${shellQuote(tokenValue)}`;
  const mountCommand = `afs ws mount ${shellQuote(targetWorkspace)} <directory>`;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || disabled) return;
    setFormError(null);
    try {
      const token = await createToken.mutateAsync({
        workspaceId,
        name: name.trim() || undefined,
        capability: permission,
        expiresAt: expiryToTimestamp(expiry),
      });
      setCreatedToken(token);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to create mount token.",
      );
    }
  }

  function copy(text: string, label: string) {
    if (text === "") return;
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(label);
      window.setTimeout(() => setCopied(null), 1800);
    });
  }

  if (disabled) {
    return (
      <NoticeCard>
        <NoticeTitle>Save Agent Workspace first</NoticeTitle>
        <NoticeBody>
          Mount tokens are available after the Agent Workspace has an ID.
        </NoticeBody>
      </NoticeCard>
    );
  }

  return (
    <TokenStack>
      <SectionHeader>
        <div>
          <Typography.Heading component="h3" size="XS" style={{ margin: 0 }}>
            Mount tokens
          </Typography.Heading>
          <Typography.Body color="secondary" component="p" style={{ margin: 0 }}>
            Scoped CLI access for this Agent Workspace.
          </Typography.Body>
        </div>
      </SectionHeader>

      <FormGrid onSubmit={submit}>
        <TwoColumnFields>
          <Field>
            Name
            <TextInput
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="e.g. CI mount, contractor laptop"
            />
          </Field>
          <Field>
            Permission
            <Select
              options={[
                { value: "mount-rw", label: "Read / write" },
                { value: "mount-ro", label: "Read only" },
              ]}
              value={permission}
              onChange={(next) =>
                setPermission(next as AFSCLIAccessTokenCapability)
              }
            />
          </Field>
        </TwoColumnFields>

        <TwoColumnFields>
          <Field>
            Expires
            <Select
              options={[
                { value: "24h", label: "24 hours" },
                { value: "7d", label: "7 days" },
                { value: "30d", label: "30 days" },
                { value: "never", label: "No expiry" },
              ]}
              value={expiry}
              onChange={(next) => setExpiry(next as ExpiryOption)}
            />
          </Field>
          <ActionField>
            <Button size="medium" type="submit" disabled={pending}>
              {pending ? "Creating..." : "Create mount token"}
            </Button>
          </ActionField>
        </TwoColumnFields>

        {formError ? <DialogError role="alert">{formError}</DialogError> : null}
      </FormGrid>

      {createdToken ? (
        <CreatedPanel>
          <CreatedHeader>
            <CreatedTitle>Mount token created</CreatedTitle>
            <CreatedMeta>
              {createdToken.capability === "mount-ro"
                ? "Read only"
                : "Read / write"}{" "}
              · {createdToken.expiresAt ? expiryLabel(createdToken.expiresAt) : "No expiry"}
            </CreatedMeta>
          </CreatedHeader>

          <SnippetBlock>
            <SnippetLabel>Token</SnippetLabel>
            <CodeBlock>{tokenValue || "(not returned)"}</CodeBlock>
            <SnippetActions>
              <Button
                size="small"
                variant="secondary-fill"
                onClick={() => copy(tokenValue, "token")}
                disabled={tokenValue === ""}
              >
                {copied === "token" ? "Copied" : "Copy token"}
              </Button>
            </SnippetActions>
          </SnippetBlock>

          <SnippetBlock>
            <SnippetLabel>Login</SnippetLabel>
            <CodeBlock>{loginCommand || "(token not returned)"}</CodeBlock>
            <SnippetActions>
              <Button
                size="small"
                variant="secondary-fill"
                onClick={() => copy(loginCommand, "login")}
                disabled={loginCommand === ""}
              >
                {copied === "login" ? "Copied" : "Copy login"}
              </Button>
            </SnippetActions>
          </SnippetBlock>

          <SnippetBlock>
            <SnippetLabel>Mount</SnippetLabel>
            <CodeBlock>{mountCommand}</CodeBlock>
            <SnippetActions>
              <Button
                size="small"
                variant="secondary-fill"
                onClick={() => copy(mountCommand, "mount")}
              >
                {copied === "mount" ? "Copied" : "Copy mount"}
              </Button>
            </SnippetActions>
          </SnippetBlock>
        </CreatedPanel>
      ) : null}
    </TokenStack>
  );
}

function expiryToTimestamp(value: ExpiryOption) {
  if (value === "never") return undefined;
  const now = Date.now();
  const hours =
    value === "24h" ? 24 : value === "7d" ? 7 * 24 : 30 * 24;
  return new Date(now + hours * 60 * 60 * 1000).toISOString();
}

function expiryLabel(raw: string) {
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  return date.toLocaleString();
}

function shellQuote(value: string) {
  return `'${value.replaceAll("'", "'\\''")}'`;
}

const TokenStack = styled.div`
  display: grid;
  gap: 18px;
  max-width: 880px;
`;

const SectionHeader = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
`;

const ActionField = styled.div`
  display: flex;
  align-items: flex-end;
  justify-content: flex-start;
  min-height: 72px;
`;

const CreatedPanel = styled.div`
  display: grid;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--afs-line);
  border-radius: 12px;
  background: var(--afs-panel);

  [data-skin="situation-room"] && {
    border-radius: var(--afs-r-2);
    border-color: var(--afs-line-strong);
    background: var(--afs-bg-1);
  }
`;

const CreatedHeader = styled.div`
  display: grid;
  gap: 4px;
`;

const CreatedTitle = styled.div`
  color: var(--afs-ink);
  font-size: 14px;
  font-weight: 700;
`;

const CreatedMeta = styled.div`
  color: var(--afs-muted);
  font-size: 13px;
`;

const SnippetBlock = styled.div`
  display: grid;
  gap: 8px;
`;

const SnippetLabel = styled.span`
  color: var(--afs-ink-soft);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
`;

const CodeBlock = styled.pre`
  margin: 0;
  max-height: 180px;
  overflow: auto;
  padding: 12px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  font-family: var(--afs-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
`;

const SnippetActions = styled.div`
  display: flex;
  justify-content: flex-end;
`;
