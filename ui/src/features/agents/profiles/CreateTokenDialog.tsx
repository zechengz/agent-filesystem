import { Button, Select } from "@redis-ui/components";
import { useState } from "react";
import type { FormEvent } from "react";
import styled from "styled-components";
import {
  DialogActions,
  DialogBody,
  DialogCard,
  DialogCloseButton,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  Field,
  FormGrid,
  TextInput,
} from "../../../components/afs-kit";
import type { CreateTokenInput, TokenScope } from "./types";

type Props = {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (input: CreateTokenInput) => void;
};

const SCOPE_LIST: Array<{ k: TokenScope; label: string; desc: string }> = [
  { k: "read", label: "read", desc: "List, fetch and search files" },
  { k: "write", label: "write", desc: "Create, modify and delete files" },
  { k: "snapshot", label: "snapshot", desc: "Create / restore checkpoints" },
  { k: "admin", label: "admin", desc: "Manage workspaces and tokens" },
];

export function CreateTokenDialog({ isOpen, onClose, onCreate }: Props) {
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<Record<TokenScope, boolean>>({
    read: true,
    write: true,
    snapshot: false,
    admin: false,
  });
  const [expiry, setExpiry] = useState("never");

  function toggle(k: TokenScope) {
    setScopes((s) => ({ ...s, [k]: !s[k] }));
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onCreate({
      name: name.trim(),
      scopes: (Object.keys(scopes) as TokenScope[]).filter((k) => scopes[k]),
    });
    setName("");
    setScopes({ read: true, write: true, snapshot: false, admin: false });
    setExpiry("never");
  }

  if (!isOpen) return null;

  return (
    <DialogOverlay
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <DialogCard>
        <DialogHeader>
          <div>
            <DialogTitle>Generate a new token</DialogTitle>
            <DialogBody>
              Tokens are revealed once. Pick a name you'll recognize and the
              scopes this agent actually needs.
            </DialogBody>
          </div>
          <DialogCloseButton onClick={onClose}>&times;</DialogCloseButton>
        </DialogHeader>

        <FormGrid onSubmit={submit}>
          <Field>
            Token name
            <TextInput
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="e.g. Production CI"
              autoFocus
            />
          </Field>

          <Field>
            Scopes
            <ScopeGrid>
              {SCOPE_LIST.map((scope) => (
                <ScopeOption key={scope.k} $selected={scopes[scope.k]}>
                  <input
                    type="checkbox"
                    checked={scopes[scope.k]}
                    onChange={() => toggle(scope.k)}
                  />
                  <ScopeLabel>
                    <ScopeName>{scope.label}</ScopeName>
                    <ScopeHint>{scope.desc}</ScopeHint>
                  </ScopeLabel>
                </ScopeOption>
              ))}
            </ScopeGrid>
          </Field>

          <Field>
            Expires
            <Select
              options={[
                { value: "30", label: "30 days" },
                { value: "90", label: "90 days" },
                { value: "365", label: "1 year" },
                { value: "never", label: "Never" },
              ]}
              value={expiry}
              onChange={(next) => setExpiry(next)}
            />
          </Field>

          <DialogActions style={{ justifyContent: "flex-end", marginTop: 8 }}>
            <Button
              size="medium"
              type="button"
              variant="secondary-fill"
              onClick={onClose}
            >
              Cancel
            </Button>
            <Button size="medium" type="submit">
              Generate token
            </Button>
          </DialogActions>
        </FormGrid>
      </DialogCard>
    </DialogOverlay>
  );
}

const ScopeGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;

  @media (max-width: 520px) {
    grid-template-columns: 1fr;
  }
`;

const ScopeOption = styled.label<{ $selected: boolean }>`
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid
    ${({ $selected }) =>
      $selected ? "var(--afs-accent)" : "var(--afs-line)"};
  background: ${({ $selected }) =>
    $selected ? "var(--afs-accent-soft)" : "var(--afs-panel)"};
  cursor: pointer;
  transition: border-color 140ms ease, background 140ms ease;

  input {
    margin-top: 2px;
    accent-color: var(--afs-accent);
  }
`;

const ScopeLabel = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
`;

const ScopeName = styled.span`
  color: var(--afs-ink);
  font-size: 13.5px;
  font-weight: 700;
`;

const ScopeHint = styled.span`
  color: var(--afs-muted);
  font-size: 12px;
  line-height: 1.5;
`;
