'use client';

import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import {
  AutoAwesome, Dashboard, FactCheck, Hub, Language, PhoneIphone, RocketLaunch,
  Storefront,
} from '@mui/icons-material';
import { A11y, Keyboard, Navigation, Pagination } from 'swiper/modules';
import { Swiper, SwiperSlide } from 'swiper/react';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';

type TemplateResource = {
  title: string;
  type: string;
  img: string;
  source: string;
  desc: string;
  stack: string;
  connectors: string[];
  gates: string[];
  prompt: string;
};

const categories = [
  { label: 'All', icon: <Dashboard fontSize="small" /> },
  { label: 'Websites', icon: <Language fontSize="small" /> },
  { label: 'Apps', icon: <AutoAwesome fontSize="small" /> },
  { label: 'Commerce', icon: <Storefront fontSize="small" /> },
  { label: 'Mobile/PWA', icon: <PhoneIphone fontSize="small" /> },
];

const resources: TemplateResource[] = [
  {
    title: 'Aiforge AI SaaS',
    type: 'Apps',
    img: '/templates/aiforge-hero.jpg',
    source: 'templates/aiforge',
    desc: 'AI product landing, integrations, service pages, blog, pricing-ready sections.',
    stack: 'HTML + SCSS + JS',
    connectors: ['OpenAI', 'Slack', 'Drive'],
    gates: ['UX', 'SEO', 'Launch'],
    prompt: 'Use the local Aiforge template as the visual foundation for an AI SaaS app with landing pages, integrations, pricing, blog, onboarding, and production launch checks.',
  },
  {
    title: 'Allstore Commerce',
    type: 'Commerce',
    img: '/templates/allstore-slide.jpg',
    source: 'templates/allstore-html-template/html',
    desc: 'Catalog, sale hero, category navigation, product grids, cart-ready storefront.',
    stack: 'HTML commerce',
    connectors: ['Stripe', 'Shopify', 'CMS'],
    gates: ['Checkout', 'Catalog', 'SEO'],
    prompt: 'Use the local Allstore HTML template as the foundation for a commerce storefront with catalog pages, product detail, cart, checkout, order states, and CMS-ready content.',
  },
  {
    title: 'Davies Agency System',
    type: 'Websites',
    img: '/templates/davies-demo.jpg',
    source: 'templates/davies-mainfiles/davies',
    desc: 'Dark agency website with portfolio demos, services, process, pricing, and contact flow.',
    stack: 'HTML portfolio',
    connectors: ['CRM', 'Analytics', 'Email'],
    gates: ['Copy', 'Portfolio', 'Performance'],
    prompt: 'Use the local Davies template as the base for a premium agency website with portfolio demos, service sections, process, pricing, contact flows, analytics, and SEO checks.',
  },
  {
    title: 'Codec Mobile Kit',
    type: 'Mobile/PWA',
    img: '/templates/codec-mobile.png',
    source: 'templates/codec-mobile/codec',
    desc: 'Mobile-first UI kit with color variants, app screens, onboarding, and PWA affordances.',
    stack: 'Mobile HTML kit',
    connectors: ['Push', 'Auth', 'Storage'],
    gates: ['Responsive', 'PWA', 'Touch'],
    prompt: 'Use the local Codec mobile template as the foundation for a mobile-first PWA with onboarding, app navigation, profile screens, push-ready flows, and touch ergonomics.',
  },
  {
    title: 'Blix Portfolio Mobile',
    type: 'Mobile/PWA',
    img: '/templates/blix-mobile.png',
    source: 'templates/blix/blix',
    desc: 'Compact portfolio mobile shell with multiple color themes and swipe-first pages.',
    stack: 'Mobile portfolio',
    connectors: ['CMS', 'Email', 'Analytics'],
    gates: ['Navigation', 'Content', 'Mobile UX'],
    prompt: 'Use the local Blix mobile template as the foundation for a mobile portfolio PWA with themed variants, project pages, contact flows, and CMS-ready portfolio content.',
  },
  {
    title: 'Varius Experience Site',
    type: 'Websites',
    img: '/templates/varius-mobile.jpg',
    source: 'templates/varius-mobile/varius',
    desc: 'Multi-vertical mobile web templates for music, restaurant, wedding, beauty, and more.',
    stack: 'Mobile web suite',
    connectors: ['Booking', 'Maps', 'Email'],
    gates: ['Content model', 'Forms', 'Mobile polish'],
    prompt: 'Use the local Varius template suite as the foundation for a polished mobile web experience with vertical-specific pages, booking/contact flows, maps, and content sections.',
  },
];

export default function ResourcesPage() {
  return (
    <RequireAuth>
      <ResourcesInner />
    </RequireAuth>
  );
}

function ResourcesInner() {
  const { user, logout } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [query, setQuery] = useState('');
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [category, setCategory] = useState('All');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return resources.filter((item) => {
      if (category !== 'All' && item.type !== category) return false;
      if (!q) return true;
      return [
        item.title,
        item.type,
        item.desc,
        item.stack,
        item.source,
        item.connectors.join(' '),
      ].join(' ').toLowerCase().includes(q);
    });
  }, [category, query]);

  function useTemplate(item: TemplateResource) {
    window.localStorage.setItem('ironflyer.pendingIdea', [
      item.prompt,
      '',
      `Template: ${item.title}`,
      `Local source: ${item.source}.`,
      `Stack direction: ${item.stack}.`,
      `Suggested connectors: ${item.connectors.join(', ')}.`,
      `Required checks: ${item.gates.join(', ')}.`,
    ].join('\n'));
    window.location.href = '/app';
  }

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Resources"
        title="Real templates, ready to remix"
        subtitle="A curated carousel backed by the local template library. Pick a source package, seed the project prompt, and continue in the builder."
      />

      <Stack direction="row" spacing={0.8} useFlexGap flexWrap="wrap" sx={{ mb: 2.2 }}>
        {categories.map((item) => (
          <Chip
            key={item.label}
            icon={item.icon}
            label={item.label}
            onClick={() => setCategory(item.label)}
            sx={{
              borderRadius: '8px',
              bgcolor: category === item.label ? tokens.color.accent.lime : '#fffaf1',
              color: tokens.color.text.inverse,
              border: `1px solid ${category === item.label ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)'}`,
              '& .MuiChip-icon': { color: 'inherit' },
            }}
          />
        ))}
      </Stack>

      <Surface sx={{ p: { xs: 1.4, md: 1.8 }, mb: 1.7 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr 1fr' }, gap: 1.2 }}>
          <Insight icon={<RocketLaunch />} title="Local source packages" text="Each card points the agent at a real template folder, not a generic marketplace placeholder." />
          <Insight icon={<Hub />} title="Connector-aware prompts" text="The selected template carries likely services and integrations into the project brief." />
          <Insight icon={<FactCheck />} title="Reviewable gates" text="Projects start with explicit UX, content, SEO, responsive, or commerce checks." />
        </Box>
      </Surface>

      {visible.length > 0 ? (
        <Box sx={swiperWrapSx}>
          <Swiper
            modules={[Navigation, Pagination, Keyboard, A11y]}
            navigation
            pagination={{ clickable: true }}
            keyboard={{ enabled: true }}
            spaceBetween={14}
            slidesPerView={1.03}
            breakpoints={{
              720: { slidesPerView: view === 'list' ? 1.45 : 2.05, spaceBetween: 16 },
              1120: { slidesPerView: view === 'list' ? 2.05 : 3.05, spaceBetween: 16 },
            }}
          >
            {visible.map((item) => (
              <SwiperSlide key={item.title}>
                <TemplateCard item={item} onUse={() => useTemplate(item)} />
              </SwiperSlide>
            ))}
          </Swiper>
        </Box>
      ) : (
        <Surface sx={{ p: 4, mt: 1.5, textAlign: 'center' }}>
          <Typography variant="h6">No templates match this search</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Try another category or clear the workspace search field.
          </Typography>
        </Surface>
      )}

      <Surface sx={{ p: { xs: 1.4, md: 1.7 }, mt: 1.8 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1.2}>
          <Box>
            <Typography variant="subtitle2">Template inventory</Typography>
            <Typography variant="body2" color="text.secondary">
              Showing {visible.length} source package{visible.length === 1 ? '' : 's'} from /templates.
            </Typography>
          </Box>
          <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap">
            {visible.slice(0, 4).map((item) => (
              <Chip key={item.source} label={item.source.replace('templates/', '')} size="small" sx={metaChipSx} />
            ))}
          </Stack>
        </Stack>
      </Surface>
    </AppShell>
  );
}

function TemplateCard({ item, onUse }: { item: TemplateResource; onUse: () => void }) {
  return (
    <Surface sx={{
      height: '100%',
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
      bgcolor: '#fffaf1',
    }}>
      <Box sx={{
        position: 'relative',
        height: { xs: 198, md: 220 },
        bgcolor: '#111',
        overflow: 'hidden',
      }}>
        <Box component="img" src={item.img} alt="" sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        <Box sx={{
          position: 'absolute',
          inset: 0,
          background: 'linear-gradient(180deg, rgba(17,17,17,0.02), rgba(17,17,17,0.68))',
        }} />
        <Stack direction="row" spacing={0.6} sx={{ position: 'absolute', left: 12, right: 12, bottom: 12 }} useFlexGap flexWrap="wrap">
          <Chip label={item.type} size="small" sx={floatingChipSx} />
          <Chip label={item.stack} size="small" sx={floatingChipSx} />
        </Stack>
      </Box>

      <Box sx={{ p: 1.8, display: 'flex', flexDirection: 'column', flex: 1 }}>
        <Typography variant="h6" sx={{ lineHeight: 1.08 }}>{item.title}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.7 }}>{item.desc}</Typography>

        <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap" sx={{ mt: 1.2 }}>
          {item.connectors.map((connector) => (
            <Chip key={connector} label={connector} size="small" sx={metaChipSx} />
          ))}
        </Stack>

        <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap" sx={{ mt: 1 }}>
          {item.gates.map((gate) => (
            <Chip key={gate} label={gate} size="small" sx={gateChipSx} />
          ))}
        </Stack>

        <Typography variant="caption" color="text.secondary" sx={{ mt: 1.3, display: 'block', wordBreak: 'break-word' }}>
          Source: {item.source}
        </Typography>

        <Button variant="contained" size="small" sx={{ mt: 'auto', alignSelf: 'flex-start', pt: 0.72 }} onClick={onUse}>
          Use template
        </Button>
      </Box>
    </Surface>
  );
}

function Insight({ icon, title, text }: { icon: React.ReactNode; title: string; text: string }) {
  return (
    <Stack direction="row" spacing={1.1} alignItems="flex-start">
      <Box sx={{ width: 34, height: 34, borderRadius: '8px', display: 'grid', placeItems: 'center', bgcolor: 'rgba(17,17,17,0.08)' }}>
        {icon}
      </Box>
      <Box>
        <Typography variant="subtitle2">{title}</Typography>
        <Typography variant="caption" color="text.secondary">{text}</Typography>
      </Box>
    </Stack>
  );
}

const swiperWrapSx = {
  mx: { xs: -0.4, md: 0 },
  pb: 0.5,
  '& .swiper': {
    pb: 4.8,
    px: { xs: 0.4, md: 0.1 },
  },
  '& .swiper-slide': {
    height: 'auto',
    display: 'flex',
  },
  '& .swiper-button-prev, & .swiper-button-next': {
    width: 34,
    height: 34,
    top: { xs: 108, md: 122 },
    display: { xs: 'none', md: 'flex' },
    borderRadius: '8px',
    bgcolor: '#111',
    color: tokens.color.accent.lime,
    boxShadow: '0 10px 28px rgba(17,17,17,0.24)',
    '&:after': { fontSize: 14, fontWeight: 900 },
    '&.swiper-button-disabled': { opacity: 0, pointerEvents: 'none' },
  },
  '& .swiper-pagination-bullet': {
    width: 8,
    height: 8,
    bgcolor: 'rgba(17,17,17,0.35)',
    opacity: 1,
  },
  '& .swiper-pagination-bullet-active': {
    width: 24,
    borderRadius: '999px',
    bgcolor: tokens.color.accent.lime,
  },
};

const floatingChipSx = {
  borderRadius: '6px',
  bgcolor: 'rgba(248,244,236,0.92)',
  border: '1px solid rgba(255,255,255,0.34)',
  color: tokens.color.text.inverse,
  fontWeight: 800,
};

const metaChipSx = {
  borderRadius: '6px',
  bgcolor: '#fffaf1',
  border: '1px solid rgba(17,17,17,0.12)',
  color: '#514a41',
};

const gateChipSx = {
  borderRadius: '6px',
  bgcolor: 'rgba(229,255,0,0.18)',
  border: '1px solid rgba(17,17,17,0.1)',
  color: tokens.color.text.inverse,
  fontWeight: 800,
};
