'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  AutoAwesome, Dashboard, Language, PhoneIphone, Storefront,
} from '@mui/icons-material';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { tokens } from '../../../lib/theme';
import { Surface } from '../workspace-shell';
import { TEMPLATES, TEMPLATE_COUNT, type TemplateMeta } from './templates.generated';

const PAGE_SIZE = 24;

const categories = [
  { label: 'All', icon: <Dashboard fontSize="small" /> },
  { label: 'Websites', icon: <Language fontSize="small" /> },
  { label: 'Apps', icon: <AutoAwesome fontSize="small" /> },
  { label: 'Commerce', icon: <Storefront fontSize="small" /> },
  { label: 'Mobile/PWA', icon: <PhoneIphone fontSize="small" /> },
];

const detailedCategories = Array.from(new Set(TEMPLATES.map((t) => t.category))).sort();
const resources: TemplateMeta[] = TEMPLATES;

export function TemplateGallery({ query, view }: { query: string; view: 'grid' | 'list' }) {
  const [category, setCategory] = useState('All');
  const [detailedCategory, setDetailedCategory] = useState('All');
  const [visibleLimit, setVisibleLimit] = useState(PAGE_SIZE);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return resources.filter((item) => {
      if (category !== 'All' && item.type !== category) return false;
      if (detailedCategory !== 'All' && item.category !== detailedCategory) return false;
      if (!q) return true;
      return [
        item.title,
        item.type,
        item.category,
        item.subtitle,
        item.stack,
        item.source,
        item.tags.join(' '),
      ].join(' ').toLowerCase().includes(q);
    });
  }, [category, detailedCategory, query]);

  useEffect(() => {
    setVisibleLimit(PAGE_SIZE);
  }, [category, detailedCategory, query, view]);

  const visibleSlice = visible.slice(0, visibleLimit);

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
    <>
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
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: view === 'grid'
            ? { xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)', lg: 'repeat(4, 1fr)' }
            : { xs: '1fr', lg: 'repeat(2, 1fr)' },
          gap: 1.6,
          mt: 0.5,
          contentVisibility: 'auto',
          containIntrinsicSize: '900px',
        }}>
          {visibleSlice.map((item, index) => (
            <TemplateCard
              key={item.slug}
              item={item}
              priority={index < 4}
              onUse={() => useTemplate(item)}
            />
          ))}
        </Box>
      ) : (
        <Surface sx={{ p: 4, mt: 1.5, textAlign: 'center' }}>
          <Typography variant="h6" sx={{ fontWeight: 900 }}>No matching templates</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Try another category or clear the search field.
          </Typography>
        </Surface>
      )}

      {visibleLimit < visible.length && (
        <Stack alignItems="center" sx={{ mt: 1.8 }}>
          <Button variant="outlined" onClick={() => setVisibleLimit((n) => n + PAGE_SIZE)}>
            Load more templates
          </Button>
        </Stack>
      )}

      <Surface sx={{ p: { xs: 1.4, md: 1.7 }, mt: 1.8 }}>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1.2}>
          <Box>
            <Typography variant="subtitle2">Available templates</Typography>
            <Typography variant="body2" color="text.secondary">
              Showing {Math.min(visibleLimit, visible.length)} of {visible.length} matching templates.
            </Typography>
          </Box>
          <Stack direction="row" spacing={0.6} useFlexGap flexWrap="wrap">
            {visible.slice(0, 4).map((item) => (
              <Chip key={item.source} label={item.source.replace('templates/sites/', '')} size="small" sx={metaChipSx} />
            ))}
          </Stack>
        </Stack>
      </Surface>
    </>
  );
}

function TemplateCard({ item, priority, onUse }: { item: TemplateMeta; priority: boolean; onUse: () => void }) {
  const swatch = item.palette;
  return (
    <Surface sx={{
      height: '100%',
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
      bgcolor: '#fffaf1',
      contentVisibility: 'auto',
      containIntrinsicSize: '360px',
    }}>
      <Box sx={{
        position: 'relative',
        height: { xs: 198, md: 220 },
        bgcolor: swatch.bg,
        overflow: 'hidden',
      }}>
        <Box
          component="img"
          src={item.previewImage}
          alt={`${item.title} preview`}
          loading={priority ? 'eager' : 'lazy'}
          decoding="async"
          fetchPriority={priority ? 'high' : 'low'}
          sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
        />
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
