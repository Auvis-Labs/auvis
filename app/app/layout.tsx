import type { Metadata } from "next";
import localFont from "next/font/local";
import { Analytics } from "@vercel/analytics/next";
import "./globals.css";

export const bricolage = localFont({
  src: [
    {
      path: "../node_modules/@fontsource-variable/bricolage-grotesque/files/bricolage-grotesque-latin-wght-normal.woff2",
      weight: "400 800",
      style: "normal",
    },
  ],
  variable: "--font-bricolage",
  display: "swap",
});

export const dmSans = localFont({
  src: [
    {
      path: "../node_modules/@fontsource-variable/dm-sans/files/dm-sans-latin-wght-normal.woff2",
      weight: "400 600",
      style: "normal",
    },
  ],
  variable: "--font-dm-sans",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Auvis - In-Flight Data Governance for AI Pipelines",
  description:
    "Policy enforcement, PII detection, and compliance for AI ingestion pipelines.",
  icons: {
    icon: [
      { url: "/auvis-final.svg", type: "image/svg+xml" },
    ],
    shortcut: ["/auvis-final.svg"],
    apple: [
      { url: "/auvis-final.svg", type: "image/svg+xml" },
    ],
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`${bricolage.variable} ${dmSans.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        {children}
        <Analytics />
      </body>
    </html>
  );
}
