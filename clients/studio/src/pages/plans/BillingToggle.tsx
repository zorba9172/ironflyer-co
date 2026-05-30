import { Box, Chip, Typography } from '@mui/material';

// ─────────────────────────────────────────────────────────────────────────
// Billing cadence switch (Linear/Stripe pattern): a single glass pill with a
// sliding neon thumb. Annual is the default and surfaces the savings inline —
// the highest-ROI element on a pricing page. Presentation only; the selected
// cadence drives the derived price the cards render. Every value reads from the
// theme — no raw color/size literals.
// ─────────────────────────────────────────────────────────────────────────

export type BillingCadence = 'monthly' | 'annual';

const OPTIONS: { value: BillingCadence; label: string }[] = [
  { value: 'monthly', label: 'Monthly' },
  { value: 'annual', label: 'Annual' },
];

export function BillingToggle({
  value,
  onChange,
  savingsPct,
}: {
  value: BillingCadence;
  onChange: (next: BillingCadence) => void;
  savingsPct: number;
}) {
  const annualActive = value === 'annual';

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 1.5, flexWrap: 'wrap' }}>
      <Box
        role="tablist"
        aria-label="Billing cadence"
        sx={(theme) => ({
          position: 'relative',
          display: 'inline-flex',
          p: 0.5,
          borderRadius: theme.studio.radius.pill,
          backgroundColor: theme.palette.cardBg,
          border: `1px solid ${theme.palette.cardBorder}`,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        })}
      >
        {/* Sliding neon thumb behind the active label. */}
        <Box
          aria-hidden
          sx={(theme) => ({
            position: 'absolute',
            top: 4,
            bottom: 4,
            left: 4,
            width: 'calc(50% - 4px)',
            borderRadius: theme.studio.radius.pill,
            backgroundImage: theme.studio.gradient.cta,
            boxShadow: `0 8px 22px ${theme.studio.neon.violet}57`,
            transform: annualActive ? 'translateX(100%)' : 'translateX(0)',
            transition: `transform ${theme.studio.motion.base}`,
          })}
        />
        {OPTIONS.map((opt) => {
          const active = opt.value === value;
          return (
            <Box
              key={opt.value}
              role="tab"
              aria-selected={active}
              tabIndex={0}
              onClick={() => onChange(opt.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  onChange(opt.value);
                }
              }}
              sx={(theme) => ({
                position: 'relative',
                zIndex: 1,
                minWidth: 104,
                textAlign: 'center',
                px: 2.5,
                py: 1,
                cursor: 'pointer',
                userSelect: 'none',
                borderRadius: theme.studio.radius.pill,
                transition: `color ${theme.studio.motion.fast}`,
                outline: 'none',
                '&:focus-visible': { boxShadow: `0 0 0 2px ${theme.studio.neon.blue}` },
              })}
            >
              <Typography
                variant="body2"
                sx={(theme) => ({
                  fontWeight: active ? theme.typography.fontWeightBold : theme.typography.fontWeightMedium,
                  color: active ? theme.palette.common.white : theme.palette.text.secondary,
                  transition: `color ${theme.studio.motion.fast}`,
                })}
              >
                {opt.label}
              </Typography>
            </Box>
          );
        })}
      </Box>

      <Chip
        size="small"
        label={`Save ${savingsPct}%`}
        sx={(theme) => ({
          height: 26,
          fontWeight: theme.typography.fontWeightBold,
          letterSpacing: '0.01em',
          color: theme.studio.neon.success,
          backgroundColor: `${theme.studio.neon.success}1F`,
          border: `1px solid ${theme.studio.neon.success}3D`,
          opacity: annualActive ? 1 : 0.45,
          transition: `opacity ${theme.studio.motion.base}`,
        })}
      />
    </Box>
  );
}
