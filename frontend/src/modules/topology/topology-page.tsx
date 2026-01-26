// Input: ConfigFlow component, UI primitives
// Output: TopologyPage component - standalone topology visualization
// Position: Main page in topology module

import { ConfigFlow } from '@/modules/topology/config-flow'

export function TopologyPage() {
  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 min-h-0">
        <ConfigFlow />
      </div>
    </div>
  )
}
