import './global.css'

import { RootProvider } from 'fumadocs-ui/provider/next'
import { Inter } from 'next/font/google'

const inter = Inter({
  subsets: ['latin'],
})

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className={inter.className} suppressHydrationWarning>
      <head>
        <script defer src="https://u.wibus.ren/script.js" data-website-id="24b5b50a-ddb7-4354-b270-e3e4274a8bdc" />
      </head>
      <body className="flex flex-col min-h-screen">
        <RootProvider>{children}</RootProvider>
      </body>
    </html>
  )
}
