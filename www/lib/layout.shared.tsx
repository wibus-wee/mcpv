import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared'
import Image from 'next/image'

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: (
        <>
          <Image src="/appicon.png" alt="mcpv" width={22} height={22} className="rounded" />
          <span className="font-semibold">mcpv</span>
        </>
      ),
      transparentMode: 'top',
    },
  }
}
