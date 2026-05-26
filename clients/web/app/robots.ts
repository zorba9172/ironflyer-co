import type { MetadataRoute } from "next";
import { SITE } from "../src/lib/seo/site";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: ["/"],
        disallow: [
          "/cockpit",
          "/dashboard",
          "/wallet",
          "/projects",
          "/executions",
          "/execution",
          "/operator",
          "/settings",
          "/p/",
          "/login",
          "/signup",
          "/api/",
          "/auth/",
        ],
      },
    ],
    sitemap: `${SITE.url.replace(/\/$/, "")}/sitemap.xml`,
    host: SITE.url,
  };
}
