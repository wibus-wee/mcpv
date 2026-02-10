import { DocsLayout } from 'fumadocs-ui/layouts/docs'

import { baseOptions } from '@/lib/layout.shared'
import { source } from '@/lib/source'

function getSection(path: string | undefined) {
  if (!path) return 'core'
  const [dir] = path.split('/', 1)
  if (!dir) return 'core'
  return (
    {
      ui: 'ui',
      core: 'core',
    }[dir] ?? 'core'
  )
}

export default function Layout({ children }: LayoutProps<'/docs'>) {
  return (
    <DocsLayout
      tree={source.getPageTree()}
      {...baseOptions()}
      sidebar={{
        tabs: {
          transform(option, node) {
            const meta = source.getNodeMeta(node)
            if (!meta || !node.icon) return option
            const color = `var(--${getSection(meta.path)}-color, var(--color-fd-foreground))`

            return {
              ...option,
              icon: (
                <div
                  className="[&_svg]:size-full rounded-lg size-full text-(--tab-color) max-md:bg-(--tab-color)/10 max-md:border max-md:p-1.5"
                  style={
                    {
                      '--tab-color': color,
                    } as object
                  }
                >
                  {node.icon}
                </div>
              ),
            }
          },
        },
      }}
    >
      {children}
    </DocsLayout>
  )
}
