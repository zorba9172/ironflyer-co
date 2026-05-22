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

interface ResourceLink {
  title: string;
  desc: string;
  href: string;
  external?: boolean;
  icon: React.ReactNode;
  cta: string;
}

interface TemplateResource {
  title: string;
  type: string;
  img: string;
  source: string;
  desc: string;
  stack: string;
  prompt: string;
}

const categories = [
  { label: 'הכל',       icon: <Dashboard fontSize="small" /> },
  { label: 'Websites',  icon: <Language fontSize="small" /> },
  { label: 'Apps',      icon: <AutoAwesome fontSize="small" /> },
  { label: 'Commerce',  icon: <Storefront fontSize="small" /> },
  { label: 'Mobile/PWA', icon: <PhoneIphone fontSize="small" /> },
];

const linkCards: ResourceLink[] = [
  {
    title: 'תיעוד',
    desc: 'מדריכי התחלה, ארכיטקטורת הגייטים, ומפת חדרי הספקים.',
    href: '/docs',
    icon: <MenuBook />,
    cta: 'פתח תיעוד',
  },
  {
    title: 'API Reference',
    desc: 'נקודות קצה של ה־orchestrator וה־runtime, כולל ה־@ironflyer/sdk.',
    href: '/docs/api',
    icon: <Code />,
    cta: 'פתח API',
  },
  {
    title: 'גלריית תבניות',
    desc: 'אוסף תבניות מוכנות שניתן להוריד לסביבת הריצה ישירות מהפרומפט.',
    href: '/app/resources#templates',
    icon: <AutoAwesome />,
    cta: 'גלה תבניות',
  },
  {
    title: 'דף סטטוס',
    desc: 'זמינות שירותים, אירועים פתוחים, וזמני תגובה ל־30 הימים האחרונים.',
    href: 'https://status.ironflyer.dev',
    external: true,
    icon: <Timeline />,
    cta: 'דף סטטוס',
  },
  {
    title: 'קהילה ו־Discord',
    desc: 'דיון עם בונים אחרים, שיתוף פאצ׳ים, וקבלת עזרה מהירה.',
    href: 'https://discord.gg/ironflyer',
    external: true,
    icon: <ChatBubbleOutline />,
    cta: 'הצטרף',
  },
  {
    title: 'יומן שינויים',
    desc: 'גרסאות חדשות, גייטים שנוספו, ושיפורי ביצועים בזמן אמת.',
    href: '/changelog',
    icon: <Description />,
    cta: 'קרא יומן',
  },
];

const resources: TemplateResource[] = [
  {
    title: 'Aiforge AI SaaS',
    type: 'Apps',
    img: '/templates/aiforge-hero.jpg',
    source: 'templates/aiforge',
    desc: 'נחיתת AI, אינטגרציות, עמודי שירות, בלוג ותמחור מוכן לייצור.',
    stack: 'HTML + SCSS + JS',
    prompt: 'Use the local Aiforge template as the visual foundation for an AI SaaS app with landing pages, integrations, pricing, blog, onboarding, and production launch checks.',
  },
  {
    title: 'Allstore Commerce',
    type: 'Commerce',
    img: '/templates/allstore-slide.jpg',
    source: 'templates/allstore-html-template/html',
    desc: 'קטלוג, באנר מבצעים, ניווט קטגוריות, ועמודי מוצר עם רשת מעוצבת.',
    stack: 'HTML commerce',
    prompt: 'Use the local Allstore HTML template as the foundation for a commerce storefront with catalog pages, product detail, cart, checkout, order states, and CMS-ready content.',
  },
  {
    title: 'Davies Agency System',
    type: 'Websites',
    img: '/templates/davies-demo.jpg',
    source: 'templates/davies-mainfiles/davies',
    desc: 'אתר סוכנות כהה עם תיק עבודות, שירותים, תהליך, תמחור וקונטקט.',
    stack: 'HTML portfolio',
    prompt: 'Use the local Davies template as the base for a premium agency website with portfolio demos, service sections, process, pricing, contact flows, analytics, and SEO checks.',
  },
  {
    title: 'Codec Mobile Kit',
    type: 'Mobile/PWA',
    img: '/templates/codec-mobile.png',
    source: 'templates/codec-mobile/codec',
    desc: 'ערכת UI מובייל עם וריאציות צבע, מסכי אפליקציה, אונבורדינג ו־PWA.',
    stack: 'Mobile HTML kit',
    prompt: 'Use the local Codec mobile template as the foundation for a mobile-first PWA with onboarding, app navigation, profile screens, push-ready flows, and touch ergonomics.',
  },
  {
    title: 'Blix Portfolio Mobile',
    type: 'Mobile/PWA',
    img: '/templates/blix-mobile.png',
    source: 'templates/blix/blix',
    desc: 'תיק עבודות מובייל קומפקטי עם מספר ערכות צבע ועמודים swipe-first.',
    stack: 'Mobile portfolio',
    prompt: 'Use the local Blix mobile template as the foundation for a mobile portfolio PWA with themed variants, project pages, contact flows, and CMS-ready portfolio content.',
  },
  {
    title: 'Varius Experience Site',
    type: 'Websites',
    img: '/templates/varius-mobile.jpg',
    source: 'templates/varius-mobile/varius',
    desc: 'אוסף תבניות web למובייל למוזיקה, מסעדה, חתונה ויופי.',
    stack: 'Mobile web suite',
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
  const [category, setCategory] = useState('הכל');

  useEffect(() => {
    void api.listProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return resources.filter((item) => {
      if (category !== 'הכל' && item.type !== category) return false;
      if (!q) return true;
      return [item.title, item.type, item.desc, item.stack, item.source].join(' ').toLowerCase().includes(q);
    });
  }, [category, query]);

  function useTemplate(item: TemplateResource) {
    window.localStorage.setItem('ironflyer.pendingIdea', [
      item.prompt,
      '',
      `Template: ${item.title}`,
      `Use the local source package at ${item.source} as the visual and interaction reference.`,
    ].join('\n'));
    window.location.href = '/app';
  }

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} recents={projects.slice(0, 5)} onLogout={logout} query={query} setQuery={setQuery} view={view} setView={setView}>
      <PageTitle
        eyebrow="משאבים"
        title="כל מה שצריך לבנות"
        subtitle="תבניות אמיתיות, תיעוד, יומן שינויים, וקהילה — מרוכזים במקום אחד."
      />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)' }, gap: 1.4, mb: 2.4 }}>
        {linkCards.map((card) => <LinkCard key={card.title} card={card} />)}
      </Box>

      <Box id="templates">
        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'flex-start', sm: 'center' }} spacing={1.2} sx={{ mb: 1.6 }}>
          <Box>
            <Typography variant="h5" sx={{ fontWeight: 900 }}>גלריית תבניות</Typography>
            <Typography variant="body2" sx={{ color: '#686158' }}>
              בחר תבנית מקומית, השתמש בה כעוגן ויזואלי, והמשך לבנייה ישירות מהפרומפט.
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
      </Box>

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
          <Typography variant="h6" sx={{ fontWeight: 900 }}>אין תבניות תואמות</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            נסה קטגוריה אחרת או נקה את שדה החיפוש.
          </Typography>
        </Surface>
      )}

      <Surface sx={{ p: { xs: 1.4, md: 1.7 }, mt: 1.8 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1.2}>
          <Box>
            <Typography variant="subtitle2">תבניות זמינות</Typography>
            <Typography variant="body2" color="text.secondary">
              מציג {visible.length} חבילת קוד אמיתית מהתיקייה המקומית /templates.
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
        <Typography variant="h6" sx={{ lineHeight: 1.08, fontWeight: 900 }}>{item.title}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.7 }}>{item.desc}</Typography>

        <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap" sx={{ mt: 1.2 }}>
          <Chip label={item.type} size="small" sx={metaChipSx} />
          <Chip label={item.stack} size="small" sx={metaChipSx} />
        </Stack>

        <Typography variant="caption" color="text.secondary" sx={{ mt: 1.3, display: 'block', wordBreak: 'break-word' }}>
          {item.source}
        </Typography>

        <Button variant="contained" size="small" sx={{ mt: 'auto', alignSelf: 'flex-start', pt: 0.72 }} onClick={onUse}>
          השתמש בתבנית
        </Button>
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
