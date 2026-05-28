import { useEffect, useRef, type ReactNode } from 'react';
import { Fancybox as NativeFancybox } from '@fancyapps/ui';
import '@fancyapps/ui/dist/fancybox/fancybox.css';

// Wrap content containing [data-fancybox] anchors to get a themed lightbox.
// Example: <Lightbox><a data-fancybox href="/big.png"><img src="/thumb.png"/></a></Lightbox>
export function Lightbox({ children, options }: { children: ReactNode; options?: Record<string, unknown> }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const container = ref.current;
    if (!container) return;
    NativeFancybox.bind(container, '[data-fancybox]', { Hash: false, ...options });
    return () => NativeFancybox.unbind(container);
  }, [options]);
  return <div ref={ref}>{children}</div>;
}
