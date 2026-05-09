export type AgentStatus = "live" | "idle";

export type MountPersistence = "shared" | "perRun";
export type MountMode = "r" | "rw";

export type Mount = {
  wsId: string;
  mount: string;
  mode: MountMode;
};

export type Filesystem = {
  shared: Mount[];
  perRun: Mount[];
};

export type AgentProfile = {
  id: string;
  name: string;
  description: string;
  letter: string;
  color: string;
  status: AgentStatus;
  sharedCount: number;
  perRunCount: number;
  tokens: number;
  lastActive: string;
  draft?: boolean;
};

export type WorkspaceOption = {
  id: string;
  name: string;
  files: number;
  size: string;
};

export type TokenScope = "read" | "write" | "snapshot" | "admin";

export type AgentToken = {
  id: string;
  name: string;
  secret: string;
  created: string;
  lastUsed: string;
  scopes: TokenScope[];
};

export type CreateTokenInput = {
  name: string;
  scopes: TokenScope[];
};
