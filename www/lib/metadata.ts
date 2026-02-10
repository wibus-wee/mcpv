import type { Metadata } from 'next/types'

import type { Page } from './source'

export function createMetadata(override: Metadata): Metadata {
  return {
    ...override,
    openGraph: {
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      url: 'https://mcpv.wibus.ren',
      images: '/banner.png',
      siteName: 'mcpv',
      ...override.openGraph,
    },
    twitter: {
      card: 'summary_large_image',
      creator: '@wibus_wee',
      title: override.title ?? undefined,
      description: override.description ?? undefined,
      images: '/banner.png',
      ...override.twitter,
    },
  }
}

export function getPageImage(page: Page) {
  const segments = [...page.slugs, 'image.webp']

  return {
    segments,
    url: `/og/${segments.join('/')}`,
  }
}

export const baseUrl
  = process.env.NODE_ENV === 'development' || !process.env.VERCEL_PROJECT_PRODUCTION_URL
    ? new URL('http://localhost:3000')
    : new URL(`https://${process.env.VERCEL_PROJECT_PRODUCTION_URL}`)
