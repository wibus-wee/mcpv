---
name: frontend-guardrails
description: "Frontend development guardrails for this repo. Use when editing or adding frontend code to ensure consistency and maintainability."
---

# Goals

- Keep SWR keys and config centralized
- Enforce UI settings access through the hook

# SWR keys / config

- When adding a key or config, update `frontend/src/lib/swr-keys.ts` and/or `frontend/src/lib/swr-config.ts`
- When using SWR, always import keys/config from those files; never define inline keys or configs elsewhere
- If reusable logic is needed, extend those files instead of duplicating

# UI settings

- When reading or writing UI settings, use `frontend/src/hooks/use-ui-settings.ts`
- When adding a UI setting, extend the hook's types, defaults, and persistence logic; do not add a new store or access storage directly

# Example

```ts
import { swrKeys } from "@/lib/swr-keys";
import { swrConfig } from "@/lib/swr-config";
import { useUiSettings } from "@/hooks/use-ui-settings";

const settings = useUiSettings();
const key = swrKeys.userProfile();
const config = swrConfig.default;
```
