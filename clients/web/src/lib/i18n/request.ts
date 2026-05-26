import { cookies } from "next/headers";
import {
  DEFAULT_LOCALE,
  LOCALE_COOKIE,
  getContentPlugin,
  normalizeLocale,
  type ContentPlugin,
  type Locale,
} from "./content";

export async function getRequestLocale(): Promise<Locale> {
  const store = await cookies();
  return normalizeLocale(store.get(LOCALE_COOKIE)?.value ?? DEFAULT_LOCALE);
}

export async function getRequestContent(): Promise<ContentPlugin> {
  return getContentPlugin(await getRequestLocale());
}
