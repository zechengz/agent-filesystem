import type {
  AgentProfile,
  AgentToken,
  Filesystem,
  WorkspaceOption,
} from "./types";

export const initialAgents: AgentProfile[] = [
  {
    id: "agt_4f9a2c",
    name: "Claude",
    description: "Coding agent — reads our repo, writes patches to scratch",
    letter: "C",
    color: "#7c3aed",
    status: "live",
    sharedCount: 2,
    perRunCount: 1,
    tokens: 2,
    lastActive: "2 min ago",
  },
  {
    id: "agt_8d1b03",
    name: "GPT Agent",
    description: "Customer-portal triage — answers questions from docs and ML pipeline",
    letter: "G",
    color: "#10b981",
    status: "live",
    sharedCount: 3,
    perRunCount: 0,
    tokens: 1,
    lastActive: "14 min ago",
  },
  {
    id: "agt_2a77ef",
    name: "Custom Bot",
    description: "Sandboxed automation runner with ephemeral scratch",
    letter: "B",
    color: "#f59e0b",
    status: "idle",
    sharedCount: 1,
    perRunCount: 2,
    tokens: 1,
    lastActive: "3 hours ago",
  },
  {
    id: "agt_91ea14",
    name: "Indexer",
    description: "Read-only crawler that indexes ML pipeline outputs",
    letter: "X",
    color: "#3b5bdb",
    status: "idle",
    sharedCount: 1,
    perRunCount: 0,
    tokens: 0,
    lastActive: "never",
  },
];

export const availableWorkspaces: WorkspaceOption[] = [
  { id: "ws_0bc33e", name: "getting-started", files: 6, size: "6 KB" },
  { id: "ws_bcb866", name: "shared-memory", files: 7, size: "5 KB" },
  { id: "ws_a12c45", name: "customer-portal", files: 142, size: "1.2 MB" },
  { id: "ws_d77881", name: "ml-pipeline", files: 87, size: "640 KB" },
  { id: "ws_e09a30", name: "docs-site", files: 56, size: "2.1 MB" },
  { id: "ws_fc5511", name: "scratch", files: 0, size: "0 KB" },
];

export const sampleFilesystems: Record<string, Filesystem> = {
  agt_4f9a2c: {
    shared: [
      { wsId: "ws_bcb866", mount: "/shared-memory", mode: "rw" },
      { wsId: "ws_e09a30", mount: "/docs", mode: "r" },
    ],
    perRun: [{ wsId: "ws_fc5511", mount: "/scratch", mode: "rw" }],
  },
  agt_8d1b03: {
    shared: [
      { wsId: "ws_a12c45", mount: "/portal", mode: "r" },
      { wsId: "ws_d77881", mount: "/ml", mode: "rw" },
      { wsId: "ws_bcb866", mount: "/shared-memory", mode: "rw" },
    ],
    perRun: [],
  },
  agt_2a77ef: {
    shared: [{ wsId: "ws_e09a30", mount: "/docs", mode: "r" }],
    perRun: [
      { wsId: "ws_fc5511", mount: "/scratch", mode: "rw" },
      { wsId: "ws_0bc33e", mount: "/run", mode: "rw" },
    ],
  },
  agt_91ea14: {
    shared: [{ wsId: "ws_d77881", mount: "/ml", mode: "r" }],
    perRun: [],
  },
};

export const sampleTokens: Record<string, AgentToken[]> = {
  agt_4f9a2c: [
    {
      id: "t_001",
      name: "Production",
      secret: "afs_•••••••••3a4f",
      created: "4/18",
      lastUsed: "2 min ago",
      scopes: ["read", "write"],
    },
    {
      id: "t_002",
      name: "Staging",
      secret: "afs_•••••••••8c1d",
      created: "4/22",
      lastUsed: "1 hour ago",
      scopes: ["read"],
    },
  ],
  agt_8d1b03: [
    {
      id: "t_011",
      name: "Default",
      secret: "afs_•••••••••12fe",
      created: "3/30",
      lastUsed: "14 min ago",
      scopes: ["read", "write"],
    },
  ],
  agt_2a77ef: [
    {
      id: "t_021",
      name: "CI runner",
      secret: "afs_•••••••••99ab",
      created: "4/02",
      lastUsed: "3 hours ago",
      scopes: ["read", "write"],
    },
  ],
  agt_91ea14: [],
};

export const profilePalette = [
  "#7c3aed",
  "#10b981",
  "#f59e0b",
  "#3b5bdb",
  "#0ea5e9",
  "#ef4444",
  "#db2777",
];
