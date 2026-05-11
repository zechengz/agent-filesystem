import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { WorkspaceTokensSection } from "./WorkspaceTokensSection";

const mocks = vi.hoisted(() => ({
  createToken: vi.fn(),
}));

vi.mock("@redis-ui/components", () => ({
  Button: (props: any) => <button {...props} />,
  Select: ({ options, value, onChange, ...props }: any) => (
    <select
      {...props}
      value={value}
      onChange={(event) => onChange(event.currentTarget.value)}
    >
      {options.map((option: any) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
  Typography: {
    Body: (props: any) => <p {...props} />,
    Heading: (props: any) => <h3 {...props} />,
  },
}));

vi.mock("../../../foundation/api/afs", () => ({
  getControlPlaneURL: () => "https://afs.example.com",
}));

vi.mock("../../../foundation/hooks/use-afs", () => ({
  useCreateCLIAccessTokenMutation: () => ({
    mutateAsync: mocks.createToken,
    isPending: false,
  }),
}));

afterEach(() => {
  cleanup();
});

describe("WorkspaceTokensSection", () => {
  beforeEach(() => {
    mocks.createToken.mockReset();
    mocks.createToken.mockResolvedValue({
      id: "cli-token-1",
      workspaceId: "agent-workspace-1",
      workspaceName: "Coding Agent",
      scope: "workspace:agent-workspace-1",
      capability: "mount-ro",
      token: "afs_cli_workspace_secret",
      createdAt: "2026-05-10T12:00:00Z",
    });
  });

  test("creates a workspace-scoped mount token and shows CLI commands", async () => {
    render(
      <WorkspaceTokensSection
        workspaceId="agent-workspace-1"
        workspaceName="Coding Agent"
      />,
    );

    fireEvent.change(screen.getByPlaceholderText(/ci mount/i), {
      target: { value: "CI mount" },
    });
    fireEvent.change(screen.getByLabelText(/permission/i), {
      target: { value: "mount-ro" },
    });
    fireEvent.change(screen.getByLabelText(/expires/i), {
      target: { value: "never" },
    });

    fireEvent.click(screen.getByRole("button", { name: /create mount token/i }));

    await waitFor(() => {
      expect(mocks.createToken).toHaveBeenCalledWith({
        workspaceId: "agent-workspace-1",
        name: "CI mount",
        capability: "mount-ro",
        expiresAt: undefined,
      });
    });

    expect(await screen.findByText("Mount token created")).toBeInTheDocument();
    expect(screen.getByText("afs_cli_workspace_secret")).toBeInTheDocument();
    expect(
      screen.getByText(
        "afs auth login --url 'https://afs.example.com' --access-token 'afs_cli_workspace_secret'",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("afs ws mount 'Coding Agent' <directory>"),
    ).toBeInTheDocument();
  });
});
