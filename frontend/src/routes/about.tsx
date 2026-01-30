// Input: TanStack Router, AboutPage module
// Output: About route component
// Position: /about route entry

import { createFileRoute } from '@tanstack/react-router'

import { AboutPage } from '@/modules/about/about-page'

export const Route = createFileRoute('/about')({
  component: AboutPage,
})
