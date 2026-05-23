'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import {
  AutoAwesome, ChatBubbleOutline, Code, Dashboard, Description, Language, MenuBook,
  PhoneIphone, Storefront, Timeline,
} from '@mui/icons-material';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { A11y, Keyboard, Navigation, Pagination } from 'swiper/modules';
import { Swiper, SwiperSlide } from 'swiper/react';
import { api, Project } from '../../../lib/api';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import { TEMPLATES, TEMPLATE_COUNT, type TemplateMeta } from './templates.generated';

interface ResourceLink {
  title: string;
  desc: string;
  href: string;
  external?: boolean;
  icon: React.ReactNode;
  cta: string;
}

const categories = [
  { label: 'All',       icon: <Dashboard fontSize="small" /> },
  { label: 'Websites',  icon: <Language fontSize="small" /> },
  { label: 'Apps',      icon: <AutoAwesome fontSize="small" /> },
  { label: 'Commerce',  icon: <Storefront fontSize="small" /> },
  { label: 'Mobile/PWA', icon: <PhoneIphone fontSize="small" /> },
];

const linkCards: ResourceLink[] = [
  {
    title: 'Docs',
    desc: 'Getting started guides, gate architecture, and provider routing details.',
    href: '/docs',
    icon: <MenuBook />,
    cta: 'Open docs',
  },
  {
    title: 'API Reference',
    desc: 'Orchestrator and runtime endpoints, including the @ironflyer/sdk client.',
    href: '/docs/api',
    icon: <Code />,
    cta: 'Open API',
  },
  {
    title: 'Template gallery',
    desc: 'Curated starter templates that can be pulled into the runtime directly from a prompt.',
    href: '/app/resources#templates',
    icon: <AutoAwesome />,
    cta: 'Explore templates',
  },
  {
    title: 'Status page',
    desc: 'Service availability, open incidents, and response times over the last 30 days.',
    href: 'https://status.ironflyer.dev',
    external: true,
    icon: <Timeline />,
    cta: 'Open status',
  },
  {
    title: 'Community and Discord',
    desc: 'Talk with other builders, share patches, and get fast help from the team.',
    href: 'https://discord.gg/ironflyer',
    external: true,
    icon: <ChatBubbleOutline />,
    cta: 'Join',
  },
  {
    title: 'Changelog',
    desc: 'New releases, newly added gates, and runtime performance improvements.',
    href: '/changelog',
    icon: <Description />,
    cta: 'Read changelog',
  },
];

// Detailed categories surfaced as a secondary filter (10 total — one per
// directory under templates/sites/). The top-level `type` chips above
// stay; this row lets users drill into Restaurant / Real Estate / etc.
const detailedCategories = Array.from(new Set(TEMPLATES.map((t) => t.category))).sort();

const resources: TemplateMeta[] = TEMPLATES;

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
  const [detailedCategory, setDetailedCategory] = useState('All');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return resources.filter((item) => {
      if (category !== 'All' && item.type !== category) return false;
      if (detailedCategory !== 'All' && item.category !== detailedCategory) return false;
      if (!q) return true;
      return [item.title, item.type, item.category, item.subtitle, item.stack, item.source, item.tags.join(' ')].join(' ').toLowerCase().includes(q);
    });
  }, [category, detailedCategory, query]);

  function useTemplate(item: TemplateMeta) {
    window.localStorage.setItem('ironflyer.pendingIdea', [
      item.prompt,
      '',
      `Template: ${item.title} (${item.category})`,
      `Use the local source package at ${item.source} as the visual and interaction reference.`,
    ].join('\n'));
    window.location.href = '/app';
  }

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="Resources"
        title="Everything you need to build"
        subtitle="Real templates, documentation, changelog, and community links in one place."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)' }, gap: 1.4, mb: 2.4 }}>
        {linkCards.map((card) => <LinkCard key={card.title} card={card} />)}
      </Box>

      <Box id="templates">
        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'flex-start', sm: 'center' }} spacing={1.2} sx={{ mb: 1.6 }}>
          <Box>
            <Typography variant="h5" sx={{ fontWeight: 900 }}>Template gallery</Typography>
            <Typography variant="body2" sx={{ color: '#686158' }}>
              {TEMPLATE_COUNT} original templates across {detailedCategories.length} categories. Pick one, seed the prompt, and ship through the gates.
            </Typography>
          </Box>
          <Stack direction="row" spacing={0.7} useFlexGap flexWrap="wrap">
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
        </Stack>

        <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap" sx={{ mb: 1.4 }}>
          <Chip
            label="All categories"
            onClick={() => setDetailedCategory('All')}
            size="small"
            sx={{
              borderRadius: '6px',
              bgcolor: detailedCategory === 'All' ? '#111' : 'transparent',
              color: detailedCategory === 'All' ? tokens.color.accent.lime : '#514a41',
              border: `1px solid ${detailedCategory === 'All' ? '#111' : 'rgba(17,17,17,0.14)'}`,
              fontWeight: 700,
            }}
          />
          {detailedCategories.map((label) => (
            <Chip
              key={label}
              label={label}
              size="small"
              onClick={() => setDetailedCategory(label)}
              sx={{
                borderRadius: '6px',
                bgcolor: detailedCategory === label ? '#111' : 'transparent',
                color: detailedCategory === label ? tokens.color.accent.lime : '#514a41',
                border: `1px solid ${detailedCategory === label ? '#111' : 'rgba(17,17,17,0.14)'}`,
                fontWeight: 600,
              }}
            />
          ))}
        </Stack>
      </Box>

      {visible.length > 0 ? (
        view === 'grid' ? (
          <Box sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)', lg: 'repeat(4, 1fr)' },
            gap: 1.6,
            mt: 0.5,
          }}>
            {visible.map((item) => (
              <TemplateCard key={item.slug} item={item} onUse={() => useTemplate(item)} />
            ))}
          </Box>
        ) : (
          <Box sx={swiperWrapSx}>
            <Swiper
              modules={[Navigation, Pagination, Keyboard, A11y]}
              navigation
              pagination={{ clickable: true }}
              keyboard={{ enabled: true }}
              spaceBetween={14}
              slidesPerView={1.03}
              breakpoints={{
                720: { slidesPerView: 1.45, spaceBetween: 16 },
                1120: { slidesPerView: 2.05, spaceBetween: 16 },
              }}
            >
              {visible.map((item) => (
                <SwiperSlide key={item.slug}>
                  <TemplateCard item={item} onUse={() => useTemplate(item)} />
                </SwiperSlide>
              ))}
            </Swiper>
          </Box>
        )
      ) : (
        <Surface sx={{ p: 4, mt: 1.5, textAlign: 'center' }}>
          <Typography variant="h6" sx={{ fontWeight: 900 }}>No matching templates</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Try another category or clear the search field.
          </Typography>
        </Surface>
      )}

      <Surface sx={{ p: { xs: 1.4, md: 1.7 }, mt: 1.8 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1.2}>
          <Box>
            <Typography variant="subtitle2">Available templates</Typography>
            <Typography variant="body2" color="text.secondary">
              Showing {visible.length} of {TEMPLATE_COUNT} original templates in templates/sites/.
            </Typography>
          </Box>
          <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap">
            {visible.slice(0, 4).map((item) => (
              <Chip key={item.source} label={item.source.replace('templates/sites/', '')} size="small" sx={metaChipSx} />
            ))}
          </Stack>
        </Stack>
      </Surface>
    </AppShell>
  );
}

function LinkCard({ card }: { card: ResourceLink }) {
  const inner = (
    <Surface sx={{
      p: 2.2,
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}`,
      '&:hover': { transform: 'translateY(-2px)', borderColor: 'rgba(17,17,17,0.28)' },
    }}>
      <Box sx={{
        width: 42, height: 42, borderRadius: '8px',
        bgcolor: '#fffaf1', border: '1px solid rgba(17,17,17,0.12)',
        display: 'grid', placeItems: 'center',
        color: tokens.color.text.inverse,
      }}>{card.icon}</Box>
      <Typography variant="h6" sx={{ mt: 1.4, fontWeight: 900 }}>{card.title}</Typography>
      <Typography variant="body2" sx={{ color: '#686158', mt: 0.4, flex: 1 }}>{card.desc}</Typography>
      <Button
        component="a"
        href={card.href}
        target={card.external ? '_blank' : undefined}
        rel={card.external ? 'noopener noreferrer' : undefined}
        size="small"
        variant="outlined"
        sx={{ alignSelf: 'flex-start', mt: 1.6 }}
      >
        {card.cta} {card.external ? '↗' : '→'}
      </Button>
    </Surface>
  );
  if (card.external) return inner;
  return <Link href={card.href} style={{ textDecoration: 'none', color: 'inherit' }}>{inner}</Link>;
}

function TemplateCard({ item, onUse }: { item: TemplateMeta; onUse: () => void }) {
  const swatch = item.palette;
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
        bgcolor: swatch.bg,
        overflow: 'hidden',
      }}>
        <Box component="img" src={item.previewImage} alt={`${item.title} preview`} loading="lazy" sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        <Box sx={{
          position: 'absolute',
          inset: 0,
          background: 'linear-gradient(180deg, rgba(17,17,17,0.02), rgba(17,17,17,0.68))',
        }} />
        <Stack direction="row" spacing={0.6} sx={{ position: 'absolute', left: 12, right: 12, bottom: 12 }} useFlexGap flexWrap="wrap">
          <Chip label={item.category} size="small" sx={floatingChipSx} />
          <Chip label={item.type} size="small" sx={floatingChipSx} />
        </Stack>
      </Box>

      <Box sx={{ p: 1.8, display: 'flex', flexDirection: 'column', flex: 1 }}>
        <Typography variant="h6" sx={{ lineHeight: 1.08, fontWeight: 900 }}>{item.title}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.7 }}>{item.subtitle}</Typography>

        <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap" sx={{ mt: 1.2 }}>
          {item.tags.slice(0, 3).map((tag) => (
            <Chip key={tag} label={tag} size="small" sx={metaChipSx} />
          ))}
        </Stack>

        <Stack direction="row" spacing={0.5} alignItems="center" sx={{ mt: 1.2 }}>
          <Box sx={{ width: 14, height: 14, borderRadius: '4px', bgcolor: swatch.bg, border: '1px solid rgba(17,17,17,0.16)' }} />
          <Box sx={{ width: 14, height: 14, borderRadius: '4px', bgcolor: swatch.fg, border: '1px solid rgba(17,17,17,0.16)' }} />
          <Box sx={{ width: 14, height: 14, borderRadius: '4px', bgcolor: swatch.accent, border: '1px solid rgba(17,17,17,0.16)' }} />
          <Typography variant="caption" color="text.secondary" sx={{ ml: 0.5, fontFamily: 'monospace' }}>{item.slug}</Typography>
        </Stack>

        <Stack direction="row" spacing={0.8} sx={{ mt: 'auto' }} useFlexGap flexWrap="wrap">
          <Button variant="contained" size="small" sx={{ pt: 0.72 }} onClick={onUse}>
            Use template
          </Button>
          {item.livePreview ? (
            <Button
              component="a"
              href={item.livePreview}
              target="_blank"
              rel="noopener noreferrer"
              variant="outlined"
              size="small"
              sx={{ pt: 0.72 }}
            >
              Live preview ↗
            </Button>
          ) : null}
        </Stack>
      </Box>
    </Surface>
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
