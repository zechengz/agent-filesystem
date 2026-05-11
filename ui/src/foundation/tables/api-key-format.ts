import type { AFSMCPProfile } from "../types/afs";

function formatProfile(profile: AFSMCPProfile) {
  switch (profile) {
    case "workspace-ro":
      return "Read only";
    case "workspace-rw":
      return "Read / write";
    case "workspace-rw-checkpoint":
      return "Read / write + checkpoints";
    case "admin-ro":
      return "Admin read only";
    case "admin-rw":
      return "Admin read / write";
    default:
      return profile;
  }
}

/**
 * Unified capability ladder shown across MCP and CLI rows.
 * MCP `ro` / CLI `mount-ro` → Read.
 * MCP `rw` / CLI `mount-rw` → Read + write.
 * MCP `rw-checkpoint`       → Read + write + checkpoints.
 * MCP `admin`               → Admin.
 */
export function formatCapability(
  capability?: string,
  profile?: AFSMCPProfile,
): string {
  switch (capability) {
    case "ro":
    case "mount-ro":
      return "Read";
    case "rw":
    case "mount-rw":
      return "Read + write";
    case "rw-checkpoint":
      return "Read + write + checkpoints";
    case "admin":
      return "Admin";
  }
  if (profile) return formatProfile(profile);
  return "Default";
}
