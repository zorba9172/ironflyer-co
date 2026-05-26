"use client";

import { useEffect, useMemo, useState } from "react";
import {
  DEFAULT_LOCALE,
  LOCALE_COOKIE,
  getContentPlugin,
  normalizeLocale,
  type Locale,
} from "./content";

function readCookieLocale(): Locale {
  if (typeof document === "undefined") return DEFAULT_LOCALE;
  const raw = document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${LOCALE_COOKIE}=`))
    ?.split("=")[1];
  return normalizeLocale(raw ? decodeURIComponent(raw) : DEFAULT_LOCALE);
}

export function useI18n() {
  const [locale, setLocaleState] = useState<Locale>(DEFAULT_LOCALE);

  useEffect(() => {
    setLocaleState(readCookieLocale());
  }, []);

  const setLocale = (next: Locale) => {
    document.cookie = `${LOCALE_COOKIE}=${encodeURIComponent(next)}; Path=/; Max-Age=31536000; SameSite=Lax`;
    setLocaleState(next);
  };

  const copy = useMemo(() => getContentPlugin(locale), [locale]);

  return { locale, copy, setLocale };
}
