// Input: DiscoveryService bindings, SWR, runtime status hook
// Input: Wails bindings, SWR, profile/runtime hooks
// Output: useToolsByServer hook for grouping tools by server
// Position: Data layer for tools module

import { useMemo } from "react";
import useSWR from "swr";

import type { ServerSpecDetail, ToolEntry } from "@bindings/mcpd/internal/ui";
import { DiscoveryService } from "@bindings/mcpd/internal/ui";

import {
  useProfiles,
  useProfileDetails,
  useRuntimeStatus,
} from "@/modules/config/hooks";

export interface ServerGroup {
  id: string;
  specKey: string;
  serverName: string;
  tools: ToolEntry[];
  profileNames: string[];
  hasToolData: boolean;
  specDetail?: ServerSpecDetail;
}

export function useToolsByServer() {
  const {
    data: tools,
    isLoading: toolsLoading,
    error: toolsError,
  } = useSWR<ToolEntry[]>("tools", () => DiscoveryService.ListTools());

  const {
    data: runtimeStatus,
    isLoading: runtimeLoading,
    error: runtimeError,
  } = useRuntimeStatus();
  const {
    data: profiles,
    isLoading: profilesLoading,
    error: profilesError,
  } = useProfiles();
  const {
    data: profileDetails,
    isLoading: detailsLoading,
    error: detailsError,
  } = useProfileDetails(profiles);

  const toolsBySpecKey = useMemo(() => {
    const map = new Map<string, ToolEntry[]>();
    if (!tools) return map;

    tools.forEach((tool) => {
      const specKey = tool.specKey || tool.serverName || tool.name;
      if (!specKey) return;
      const bucket = map.get(specKey);
      if (bucket) {
        bucket.push(tool);
      } else {
        map.set(specKey, [tool]);
      }
    });

    return map;
  }, [tools]);

  const serversFromProfiles = useMemo(() => {
    const map = new Map<
      string,
      { serverName: string; profiles: Set<string>; specDetail?: ServerSpecDetail }
    >();
    if (!profileDetails) return map;

    profileDetails.forEach((profile) => {
      profile.servers.forEach((server) => {
        if (!server.specKey) return;
        const entry =
          map.get(server.specKey) ?? {
            serverName: server.name,
            profiles: new Set<string>(),
            specDetail: server,
          };
        if (!entry.serverName && server.name) {
          entry.serverName = server.name;
        }
        if (!entry.specDetail) {
          entry.specDetail = server;
        }
        entry.profiles.add(profile.name);
        map.set(server.specKey, entry);
      });
    });

    return map;
  }, [profileDetails]);

  const serverMap = useMemo(() => {
    const map = new Map<string, ServerGroup>();

    const ensureServer = (
      specKey: string,
      serverName?: string,
      specDetail?: ServerSpecDetail,
    ) => {
      if (!specKey) return null;
      const existing = map.get(specKey);
      if (existing) {
        if (!existing.serverName && serverName) {
          existing.serverName = serverName;
        }
        if (!existing.specDetail && specDetail) {
          existing.specDetail = specDetail;
        }
        return existing;
      }
      const entry: ServerGroup = {
        id: specKey,
        specKey,
        serverName: serverName || specKey,
        tools: [],
        profileNames: [],
        hasToolData: false,
        specDetail,
      };
      map.set(specKey, entry);
      return entry;
    };

    serversFromProfiles.forEach((info, specKey) => {
      const entry = ensureServer(specKey, info.serverName, info.specDetail);
      if (entry) {
        entry.profileNames = Array.from(info.profiles);
      }
    });

    runtimeStatus?.forEach((status) => {
      ensureServer(status.specKey, status.serverName);
    });

    toolsBySpecKey.forEach((toolList, specKey) => {
      const entry = ensureServer(specKey);
      if (entry) {
        entry.tools = toolList;
        entry.hasToolData = true;
      }
    });

    return map;
  }, [runtimeStatus, serversFromProfiles, toolsBySpecKey]);

  const servers = useMemo(() => {
    return Array.from(serverMap.values()).sort((a, b) =>
      a.serverName.localeCompare(b.serverName)
    );
  }, [serverMap]);

  const isLoading =
    toolsLoading || profilesLoading || detailsLoading || runtimeLoading;
  const error = toolsError || profilesError || detailsError || runtimeError;

  return {
    servers,
    serverMap,
    isLoading,
    error,
    runtimeStatus,
  };
}
