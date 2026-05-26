import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "{{PROJECT_NAME}}",
  description: "An Ironflyer-finished product.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body
        style={{
          margin: 0,
          fontFamily: "Inter, system-ui, sans-serif",
          background: "#0b0b0c",
          color: "#f5f5f4",
          minHeight: "100vh",
        }}
      >
        {children}
      </body>
    </html>
  );
}
