import type { ReactNode } from 'react';
import { Box, Button, Chip, InputBase, Stack, Typography } from '@mui/material';
import { LuDownload, LuFilter, LuSearch } from 'react-icons/lu';
import { text } from '@ironflyer/design-tokens/brand';

export type StudioTableTab = {
  value: string;
  label: string;
  count?: number;
  tone?: 'default' | 'success' | 'warning' | 'error' | 'info';
};

export type StudioTableShellProps = {
  title?: ReactNode;
  subtitle?: ReactNode;
  tabs?: readonly StudioTableTab[];
  activeTab?: string;
  onTabChange?: (value: string) => void;
  actions?: ReactNode;
  showDefaultActions?: boolean;
  footer?: ReactNode;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  searchPlaceholder?: string;
  children: ReactNode;
};

const toneColor = (tone: StudioTableTab['tone']) => {
  if (tone === 'success') return 'success.main';
  if (tone === 'warning') return 'warning.main';
  if (tone === 'error') return 'error.main';
  if (tone === 'info') return 'secondary.main';
  return 'text.secondary';
};

export function StudioTableShell({
  title,
  subtitle,
  tabs,
  activeTab,
  onTabChange,
  actions,
  showDefaultActions = false,
  footer,
  searchValue,
  onSearchChange,
  searchPlaceholder = 'Search table',
  children,
}: StudioTableShellProps) {
  const hasChrome = title != null || subtitle != null || tabs?.length || actions != null || showDefaultActions || onSearchChange != null;

  if (!hasChrome && footer == null) {
    return <>{children}</>;
  }

  return (
    <Box
      sx={(theme) => ({
        border: `1px solid ${theme.palette.cardBorder}`,
        borderRadius: `${theme.studio.radius.sm}px`,
        bgcolor: 'background.paper',
        boxShadow: '0 1px 2px rgba(24,22,20,0.04)',
        overflow: 'hidden',
      })}
    >
      {hasChrome && (
        <Box sx={(theme) => ({ px: 2, pt: 1.8, pb: 1.25, borderBottom: `1px solid ${theme.palette.borderSubtle}` })}>
          <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.25} alignItems={{ xs: 'stretch', lg: 'flex-start' }}>
            {(title != null || subtitle != null) && (
              <Box sx={{ minWidth: 0, flex: 1 }}>
                {title != null && (
                  <Typography sx={{ fontSize: text.s105, fontWeight: 800, lineHeight: 1.2 }} noWrap>
                    {title}
                  </Typography>
                )}
                {subtitle != null && (
                  <Typography sx={{ mt: 0.25, color: 'text.secondary', fontSize: text.s78, lineHeight: 1.45 }}>
                    {subtitle}
                  </Typography>
                )}
              </Box>
            )}

            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              justifyContent="flex-end"
              sx={{ width: { xs: '100%', lg: 'auto' }, flexWrap: 'wrap', gap: 1 }}
            >
              {onSearchChange && (
                <Box
                  sx={(theme) => ({
                    display: 'flex',
                    alignItems: 'center',
                    gap: 0.75,
                    width: { xs: '100%', sm: 260 },
                    minWidth: 0,
                    height: 36,
                    px: 1.1,
                    borderRadius: `${theme.studio.radius.sm}px`,
                    border: `1px solid ${theme.palette.divider}`,
                    bgcolor: theme.palette.surfaceHover,
                  })}
                >
                  <LuSearch size={15} />
                  <InputBase
                    value={searchValue ?? ''}
                    onChange={(e) => onSearchChange(e.target.value)}
                    placeholder={searchPlaceholder}
                    sx={{ flex: 1, minWidth: 0, fontSize: text.s78 }}
                  />
                </Box>
              )}
              {actions ?? (showDefaultActions ? (
                <>
                  <Button size="small" variant="outlined" color="inherit" startIcon={<LuFilter size={14} />}>Filter</Button>
                  <Button size="small" variant="outlined" color="inherit" startIcon={<LuDownload size={14} />}>Export</Button>
                </>
              ) : null)}
            </Stack>
          </Stack>

          {tabs?.length ? (
            <Box
              sx={(theme) => ({
                mt: 1.4,
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                p: 0.45,
                borderRadius: `${theme.studio.radius.sm}px`,
                bgcolor: theme.palette.surfaceHover,
                overflowX: 'auto',
              })}
            >
              {tabs.map((tab) => {
                const active = tab.value === activeTab;
                return (
                  <Button
                    key={tab.value}
                    size="small"
                    onClick={() => onTabChange?.(tab.value)}
                    sx={(theme) => ({
                      flexShrink: 0,
                      minHeight: 32,
                      px: 1.25,
                      borderRadius: `${theme.studio.radius.sm}px`,
                      color: active ? 'text.primary' : 'text.secondary',
                      bgcolor: active ? 'background.paper' : 'transparent',
                      boxShadow: active ? `inset 0 0 0 1px ${theme.palette.primary.main}33, 0 1px 2px rgba(24,22,20,0.05)` : 'none',
                      fontWeight: active ? 800 : 700,
                      '&:hover': { bgcolor: active ? 'background.paper' : `${theme.palette.primary.main}0d` },
                    })}
                  >
                    <Stack component="span" direction="row" spacing={0.75} alignItems="center">
                      <Box component="span">{tab.label}</Box>
                      {typeof tab.count === 'number' && (
                        <Chip
                          component="span"
                          label={tab.count.toLocaleString()}
                          size="small"
                          sx={{
                            height: 18,
                            minWidth: 18,
                            fontSize: text.s60,
                            color: toneColor(tab.tone),
                            bgcolor: 'background.default',
                            '& .MuiChip-label': { px: 0.65 },
                          }}
                        />
                      )}
                    </Stack>
                  </Button>
                );
              })}
            </Box>
          ) : null}
        </Box>
      )}

      <Box>{children}</Box>

      {footer != null && (
        <Box sx={(theme) => ({ px: 2, py: 1.2, borderTop: `1px solid ${theme.palette.borderSubtle}`, bgcolor: 'background.default' })}>
          {typeof footer === 'string'
            ? <Typography sx={{ color: 'text.secondary', fontSize: text.s74 }}>{footer}</Typography>
            : footer}
        </Box>
      )}
    </Box>
  );
}
