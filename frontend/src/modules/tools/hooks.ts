// Input: WailsService bindings, SWR, runtime status hook
// Output: useToolsByServer hook for grouping tools by server
// Position: Data layer for tools module

import { useMemo } from "react";
import useSWR from "swr";

import { WailsService, type ToolEntry } from "@bindings/mcpd/internal/ui";

import {
  useProfiles,
  useProfileDetails,
  useRuntimeStatus,
} from "@/modules/config/hooks";

interface ServerGroup {
  id: string;
  specKey: string;
  serverName: string;
  tools: ToolEntry[];
  profileNames: string[];
  hasToolData: boolean;
}

export function useToolsByServer() {
  const {
    data: tools,
    isLoading: toolsLoading,
    error: toolsError,
  } = useSWR<ToolEntry[]>("tools", () => WailsService.ListTools());

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
      { serverName: string; profiles: Set<string> }
    >();
    if (!profileDetails) return map;

    profileDetails.forEach((profile) => {
      profile.servers.forEach((server) => {
        if (!server.specKey) return;
        const entry =
          map.get(server.specKey) ?? {
            serverName: server.name,
            profiles: new Set<string>(),
          };
        if (!entry.serverName && server.name) {
          entry.serverName = server.name;
        }
        entry.profiles.add(profile.name);
        map.set(server.specKey, entry);
      });
    });

    return map;
  }, [profileDetails]);

  const serverMap = useMemo(() => {
    const map = new Map<string, ServerGroup>();

    const ensureServer = (specKey: string, serverName?: string) => {
      if (!specKey) return null;
      const existing = map.get(specKey);
      if (existing) {
        if (!existing.serverName && serverName) {
          existing.serverName = serverName;
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
      };
      map.set(specKey, entry);
      return entry;
    };

    serversFromProfiles.forEach((info, specKey) => {
      const entry = ensureServer(specKey, info.serverName);
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
