# Analytics Events Spec

Common context:
- `route` is automatically attached to all `track()` calls when available.
- `build_channel` and `env_mode` are attached to all events (including `page_view`).

| Event | Trigger | Key Properties |
| --- | --- | --- |
| `app_launch` | App bootstrap in `frontend/src/main.tsx` | None |
| `page_view` | Route change in `frontend/src/providers/root-provider.tsx` | `url`, `title` |
| `settings_analytics_toggle` | Telemetry switch in `frontend/src/lib/analytics.ts` | `enabled` |
| `deep_link_opened` | Wails deep-link event in `frontend/src/providers/root-provider.tsx` | `path`, `has_params`, `params_count` |
| `core_start` | Start Core button in `frontend/src/modules/dashboard/dashboard-page.tsx` | `result` |
| `core_stop` | Stop/Cancel Core in `frontend/src/modules/dashboard/dashboard-page.tsx` | `result` |
| `core_restart` | Restart/Retry Core in `frontend/src/modules/dashboard/dashboard-page.tsx` | `result` |
| `core_state_refresh` | Refresh Core state in `frontend/src/modules/dashboard/dashboard-page.tsx` | `result` |
| `debug_snapshot_export` | Copy Debug flow in `frontend/src/modules/dashboard/dashboard-page.tsx` | `result` |
| `logs_filter_changed` | Log filters updated in `frontend/src/modules/logs/logs-viewer.tsx` | `filter`, `value` |
| `logs_search` | Log search input debounced in `frontend/src/modules/logs/logs-viewer.tsx` | `query_len`, `has_query` |
| `logs_clear` | Clear logs in `frontend/src/modules/logs/logs-viewer.tsx` | `log_count` |
| `logs_stream_refresh` | Restart log stream in `frontend/src/modules/logs/logs-viewer.tsx` | `result` |
| `log_row_selected` | Select a log row in `frontend/src/modules/logs/logs-viewer.tsx` | `level`, `source` |
| `log_detail_toggle` | Log detail panel open/close in `frontend/src/modules/logs/logs-viewer.tsx` | `open` |
| `logs_bottom_panel_toggle` | Bottom log panel open/close in `frontend/src/modules/logs/logs-viewer.tsx` | `open` |
| `server_search` | Server search in `frontend/src/modules/servers/servers-page.tsx` | `query_len`, `has_query`, `result_count` |
| `server_detail_opened` | Open server detail in `frontend/src/modules/servers/servers-page.tsx` | `transport`, `disabled`, `tags_count` |
| `server_tab_changed` | Switch server detail tab in `frontend/src/modules/servers/components/server-detail-drawer.tsx` | `tab` |
| `server_edit_opened` | Open add/edit server sheet in `frontend/src/modules/servers/servers-page.tsx` | `mode` |
| `server_tools_expand` | Expand/collapse tools row in `frontend/src/modules/servers/components/servers-data-table.tsx` | `expanded`, `tool_count` |
| `server_save` | Add/Edit server submit in `frontend/src/modules/servers/components/server-edit-sheet.tsx` | `mode`, `result`, `transport`, `activation_mode`, `strategy`, `tags_count`, `expose_tools_count` |
| `server_toggle_disabled` | Enable/Disable server in `frontend/src/modules/servers/hooks.ts` | `result`, `next_state` |
| `server_delete` | Delete server in `frontend/src/modules/servers/hooks.ts` | `result` |
| `server_import_opened` | Open Import Sheet in `frontend/src/modules/servers/components/import-mcp-servers-sheet.tsx` | `server_count` |
| `server_import_parse` | Parse JSON in Import Sheet | `server_count`, `error_count` |
| `server_import_apply` | Apply import in Import Sheet | `server_count`, `result` |
| `topology_viewed` | Topology layout ready in `frontend/src/modules/topology/config-flow.tsx` | `tag_count`, `server_count`, `client_count`, `instance_count` |
| `topology_node_focus` | Focus a topology node in `frontend/src/modules/topology/config-flow.tsx` | `node_type` |
| `settings_runtime_save` | Save runtime settings in `frontend/src/modules/settings/hooks/use-runtime-settings.ts` | `result`, `dirty_fields_count` |
| `settings_subagent_save` | Save SubAgent settings in `frontend/src/modules/settings/hooks/use-subagent-settings.ts` | `result`, `provider`, `enabled_tags_count`, `has_inline_api_key` |
| `settings_subagent_fetch_models` | Fetch models in `frontend/src/modules/settings/hooks/use-subagent-settings.ts` | `result`, `provider`, `model_count` |
| `settings_subagent_model_select` | Select model in `frontend/src/modules/settings/hooks/use-subagent-settings.ts` | `provider`, `model` |
| `settings_subagent_tags_change` | Change enabled tags in `frontend/src/modules/settings/components/subagent-settings-card.tsx` | `selected_count`, `unavailable_count` |
| `settings_theme_change` | Switch theme in `frontend/src/routes/settings/appearance.tsx` | `theme` |
| `plugin_search` | Plugin search in `frontend/src/modules/plugin/plugin-page.tsx` | `query_len`, `has_query`, `result_count` |
| `plugin_edit_opened` | Open add/edit plugin sheet in `frontend/src/modules/plugin/plugin-page.tsx` | `mode` |
| `plugin_save_attempted` | Plugin save attempt in `frontend/src/modules/plugin/components/plugin-edit-sheet.tsx` | `mode`, `result` |
| `connect_ide_opened` | Open Connect IDE Sheet in `frontend/src/components/common/connect-ide-sheet.tsx` | `selector_mode`, `server_count`, `tag_count` |
| `connect_ide_target_change` | Change Connect IDE target in `frontend/src/components/common/connect-ide-sheet.tsx` | `mode`, `value_source`, `has_value` |
| `connect_ide_tab_change` | Switch client tab in Connect IDE | `client` |
| `connect_ide_copy` | Copy preset in Connect IDE | `client`, `block`, `result` |
| `connect_ide_install_cursor` | Install in Cursor button | `client`, `result` |
| `plugin_install` | Placeholder | None |
| `plugin_remove` | Placeholder | None |
