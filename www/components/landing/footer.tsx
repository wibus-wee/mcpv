'use client'

import Image from 'next/image'
import Link from 'next/link'
import { AnimateInView } from '@/components/landing/animate-in-view'

const columns = [
  {
    title: 'Product',
    links: [
      { href: '/docs', label: 'Documentation' },
      { href: 'https://github.com/wibus-wee/mcpv/releases', label: 'Releases', external: true },
    ],
  },
  {
    title: 'Community',
    links: [
      { href: 'https://github.com/wibus-wee/mcpv', label: 'GitHub', external: true },
      { href: 'https://modelcontextprotocol.io/', label: 'MCP Spec', external: true },
    ],
  },
]

export function Footer() {
  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-fd-border/65 pb-10 pt-14">
      <AnimateInView className="mx-auto max-w-6xl px-6">
        <div className="grid gap-10 md:grid-cols-[1.3fr_1fr]">
          <div>
            <Link
              href="/"
              className="inline-flex items-center gap-3 transition-opacity hover:opacity-80"
            >
              <Image src="/appicon.png" alt="mcpv" width={28} height={28} className="rounded-md" />
              <span className="font-[family-name:var(--font-home-display)] text-lg font-semibold tracking-[-0.02em] text-fd-foreground">
                mcpv
              </span>
            </Link>
            <p className="mt-4 max-w-md text-sm leading-relaxed text-fd-muted-foreground">
              Elastic control plane for MCP servers with on-demand runtime,
              unified gateway routing, and operational observability built in.
            </p>
          </div>

          <div className="grid grid-cols-2 gap-6">
            {columns.map(column => (
              <div key={column.title}>
                <h3 className="text-sm font-medium text-fd-foreground">
                  {column.title}
                </h3>
                <nav className="mt-3 flex flex-col gap-2">
                  {column.links.map(link => (
                    <Link
                      key={link.label}
                      href={link.href}
                      {...(link.external && { target: '_blank', rel: 'noopener noreferrer' })}
                      className="text-sm text-fd-muted-foreground transition-colors hover:text-fd-foreground"
                    >
                      {link.label}
                    </Link>
                  ))}
                </nav>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-10 flex flex-col gap-2 border-t border-fd-border/65 pt-5 text-xs text-fd-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p>Copyright {year} mcpv</p>
          <p>Apache License 2.0</p>
        </div>
      </AnimateInView>
    </footer>
  )
}
