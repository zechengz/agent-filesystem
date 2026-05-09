import { Button, Typography } from "@redis-ui/components";
import { Trash2 } from "lucide-react";
import styled from "styled-components";
import {
  Field,
  FieldGroup,
  NoticeBody,
  NoticeCard,
  NoticeTitle,
  TextArea,
  TextInput,
  TwoColumnFields,
} from "../../../components/afs-kit";
import type { AgentProfile } from "./types";

type Props = {
  agent: AgentProfile;
  name: string;
  description: string;
  isNew: boolean;
  onChangeName: (next: string) => void;
  onChangeDescription: (next: string) => void;
  onDelete: (id: string) => void;
};

export function SettingsSection({
  agent,
  name,
  description,
  isNew,
  onChangeName,
  onChangeDescription,
  onDelete,
}: Props) {
  return (
    <Stack>
      <FieldGroup title="Identity">
        <TwoColumnFields>
          <Field>
            Display name
            <TextInput
              value={name}
              onChange={(event) => onChangeName(event.target.value)}
              placeholder='e.g. "Coding Agent"'
            />
          </Field>
          <Field>
            Agent ID
            <TextInput
              value={isNew ? "— assigned on save —" : agent.id}
              readOnly
            />
          </Field>
        </TwoColumnFields>
        <Field>
          Description
          <TextArea
            value={description}
            onChange={(event) => onChangeDescription(event.target.value)}
            placeholder="What this agent is for &mdash; e.g. reads our repo, writes patches to scratch"
          />
        </Field>
      </FieldGroup>

      {!isNew ? (
        <NoticeCard $tone="danger">
          <NoticeTitle>Danger zone</NoticeTitle>
          <NoticeBody>
            <Typography.Body
              component="p"
              color="secondary"
              style={{ margin: "0 0 12px" }}
            >
              Permanently remove this agent profile. Mounted workspaces are not
              deleted.
            </Typography.Body>
            <Button
              size="medium"
              variant="secondary-ghost"
              onClick={() => onDelete(agent.id)}
            >
              <Trash2 size={14} strokeWidth={1.75} aria-hidden="true" />
              &nbsp;Delete {agent.name || "profile"}
            </Button>
          </NoticeBody>
        </NoticeCard>
      ) : null}
    </Stack>
  );
}

const Stack = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;
