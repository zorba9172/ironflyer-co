import { useEffect, useState } from 'react';

// True only after client mount — guards DOM/canvas libs against SSR/SSG.
export function useMounted(): boolean {
  const [m, setM] = useState(false);
  useEffect(() => setM(true), []);
  return m;
}
