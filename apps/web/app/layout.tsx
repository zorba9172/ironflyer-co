import type { Metadata, Viewport } from 'next';
import { Archivo_Black, Inter } from 'next/font/google';
import Script from 'next/script';
import './globals.css';
import { Providers } from './providers';
import { PWARegister } from './pwa-register';
import { PWAInstaller } from '../components/PWAInstaller';

const displayFont = Archivo_Black({
  subsets: ['latin'],
  weight: '400',
  variable: '--font-display',
  display: 'swap',
});

const bodyFont = Inter({
  subsets: ['latin'],
  variable: '--font-body',
  display: 'swap',
});

// SEO + social card. English is the canonical product language until a
// formal localization strategy exists.
export const metadata: Metadata = {
  title: 'Ironflyer — Build, gate, and ship AI apps end-to-end',
  description:
    'Describe the app you want. Ironflyer ships it through Spec, UX, Code, Lint, Tests, Security, and Deploy gates — no credit traps, real Linux sandboxes, multi-provider routing.',
  manifest: '/manifest.webmanifest',
  appleWebApp: { capable: true, statusBarStyle: 'black-translucent', title: 'Ironflyer' },
  openGraph: {
    title: 'Ironflyer — AI Product Finisher',
    description:
      'Vibe-code apps the finisher way: gated end-to-end with real sandboxes, transparent budget, and multi-provider routing.',
    siteName: 'Ironflyer',
    locale: 'en_US',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Ironflyer — AI Product Finisher',
    description:
      'Ship done, not demo. AI app builder with finisher gates, transparent margin pricing, and real cloud workspaces.',
  },
  alternates: {
    languages: {
      'en-US': '/',
    },
  },
};

export const viewport: Viewport = {
  themeColor: '#f4f0e8',
  width: 'device-width',
  initialScale: 1,
  maximumScale: 5,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${displayFont.variable} ${bodyFont.variable}`}>
      <head>
        <Script id="google-tag-manager" strategy="beforeInteractive">
          {`(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','GTM-M9PQ2TT6');`}
        </Script>
      </head>
      <body style={{ margin: 0, background: '#f4f0e8', fontFamily: 'var(--font-body)', color: '#0d0e0f' }}>
        <noscript>
          <iframe
            src="https://www.googletagmanager.com/ns.html?id=GTM-M9PQ2TT6"
            height="0"
            width="0"
            style={{ display: 'none', visibility: 'hidden' }}
          />
        </noscript>
        <Providers>{children}</Providers>
        <PWARegister />
        <PWAInstaller />
      </body>
    </html>
  );
}
