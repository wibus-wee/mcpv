import { IBM_Plex_Mono, Sora } from 'next/font/google'

import { CTA } from '@/components/landing/cta'
import { Features } from '@/components/landing/features'
import { Footer } from '@/components/landing/footer'
import { Hero } from '@/components/landing/hero'

const sora = Sora({
  subsets: ['latin'],
  variable: '--font-home-display',
})

const ibmPlexMono = IBM_Plex_Mono({
  subsets: ['latin'],
  weight: ['400', '500'],
  variable: '--font-home-mono',
})

export default function Page() {
  return (
    <main className={`${sora.variable} ${ibmPlexMono.variable} relative isolate overflow-hidden bg-fd-background text-fd-foreground`}>
      <Hero />
      <Features />
      <CTA />
      <Footer />
    </main>
  )
}
