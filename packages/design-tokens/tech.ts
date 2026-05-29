// Vendor / tool / framework brand colors used by the studio's table icons.
// These are the canonical brand colors each project publishes — NOT Ironflyer's
// UI palette — and they live in design-tokens so the constitutional "no raw hex
// outside tokens" rule still holds for components that render tech icons.
//
// Brands whose mark is near-black or near-white (GitHub, Vercel) are
// intentionally omitted: their icons inherit the themed `currentColor` instead,
// so they stay visible on both dark and light surfaces.

export const vendorColor: Record<string, string> = {
  go: '#00ADD8',
  react: '#61DAFB',
  typescript: '#3178C6',
  javascript: '#F7DF1E',
  python: '#3776AB',
  rust: '#DEA584',
  node: '#5FA04E',
  vite: '#646CFF',
  docker: '#2496ED',
  kubernetes: '#326CE5',
  graphql: '#E10098',
  css: '#1572B6',
  html: '#E34F26',
  // billing / infra / observability vendors
  stripe: '#635BFF',
  paddle: '#FDDD35',
  slack: '#4A154B',
  sentry: '#8D5494',
  auth0: '#EB5424',
  postgresql: '#4169E1',
  eslint: '#4B32C3',
};
