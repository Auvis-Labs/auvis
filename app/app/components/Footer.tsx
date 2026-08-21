"use client";

import React from "react";
import Link from "next/link";
import Image from "next/image";
import Blaze from "../animations/Blaze";

export default function Footer() {
  return (
    <footer className="relative w-full overflow-hidden border-t border-white/10 bg-[#0B0C11] z-10 text-white">
      <Blaze
        height={0.97}
        distortion={0.6}
        distortionScale={0.5}
        speed={1}
        sparks={0.5}
        sparkDensity={1.5}
        sparkSize={1}
        layers={4}
        smoke={0.5}
        glow={1.5}
        sparkColor={[0.7216, 0.6627, 0.9765]}
        smokeColor={[0.4824, 0.3843, 0.9882]}
        className="w-full"
      >
        <div className="relative w-full py-16 md:py-24">
          {/* Background Watermark */}
          <div className="pointer-events-none absolute right-[4%] -bottom-8 select-none text-[18vw] font-bold uppercase tracking-tighter leading-none text-white/[0.02] z-0">
            auvis
          </div>

          <div className="relative z-10 mx-auto max-w-[1240px] px-6 sm:px-8">
            {/* Top Footer Columns Grid (5 Columns) */}
            <div className="grid grid-cols-2 gap-8 pb-16 md:grid-cols-5 md:gap-12 border-b border-white/10">
              {/* Brand/Logo Column (2 cols on desktop) */}
              <div className="col-span-2 flex flex-col items-start gap-4">
                <Link
                  href="/"
                  className="flex items-center gap-3 text-white transition-opacity hover:opacity-90"
                >
                  <Image
                    src="/auvis-final.svg"
                    alt="Auvis Logo"
                    width={32}
                    height={32}
                    className="h-7 w-7"
                  />
                  <span className="text-2xl font-bold tracking-tight text-white">
                    Auvis
                  </span>
                </Link>

                <p className="max-w-[320px] text-sm font-normal leading-relaxed text-slate-300">
                  Policy enforcement and in-flight data governance for AI ingestion pipelines, vector databases, and RAG systems.
                </p>

                <div className="flex items-center gap-2 pt-2 font-mono text-xs text-slate-400">
                  <span className="inline-block h-2 w-2 rounded-full bg-emerald-400" />
                  <span>Apache 2.0 Open Source</span>
                </div>
              </div>

              {/* Col 2: Product */}
              <div className="flex flex-col gap-3">
                <span className="mb-1 text-sm font-semibold uppercase tracking-wider text-white">
                  Product
                </span>
                <a
                  href="#how-it-works"
                  className="text-sm text-slate-400 transition-colors hover:text-white"
                >
                  How it works
                </a>
                <a
                  href="#features"
                  className="text-sm text-slate-400 transition-colors hover:text-white"
                >
                  Features
                </a>
                <a
                  href="#faqs"
                  className="text-sm text-slate-400 transition-colors hover:text-white"
                >
                  FAQs
                </a>
                <a
                  href="https://github.com/cridiv/Auvis/releases"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-slate-400 transition-colors hover:text-white flex items-center gap-1"
                >
                  Releases <span className="text-xs opacity-75">↗</span>
                </a>
              </div>

              {/* Col 3: Integrations & Sinks */}
              <div className="flex flex-col gap-3">
                <span className="mb-1 text-sm font-semibold uppercase tracking-wider text-white">
                  Sinks &amp; Streams
                </span>
                <span className="text-sm text-slate-400">Postgres / pgvector</span>
                <span className="text-sm text-slate-400">Weaviate</span>
                <span className="text-sm text-slate-400">Pinecone</span>
                <span className="text-sm text-slate-400">Apache Kafka</span>
                <span className="text-sm text-slate-400">Confluence / S3</span>
              </div>

              {/* Col 4: Resources */}
              <div className="flex flex-col gap-3">
                <span className="mb-1 text-sm font-semibold uppercase tracking-wider text-white">
                  Resources
                </span>
                <a
                  href="https://github.com/cridiv/Auvis#documentation"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-slate-400 transition-colors hover:text-white flex items-center gap-1"
                >
                  Documentation <span className="text-xs opacity-75">↗</span>
                </a>
                <a
                  href="https://github.com/cridiv/Auvis/blob/main/docs/policies.md"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-slate-400 transition-colors hover:text-white flex items-center gap-1"
                >
                  Policy DSL <span className="text-xs opacity-75">↗</span>
                </a>
                <a
                  href="https://github.com/cridiv/Auvis/blob/main/docs/audit.md"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-slate-400 transition-colors hover:text-white flex items-center gap-1"
                >
                  Audit Guide <span className="text-xs opacity-75">↗</span>
                </a>
                <a
                  href="https://github.com/cridiv/Auvis"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-slate-400 transition-colors hover:text-white flex items-center gap-1"
                >
                  GitHub <span className="text-xs opacity-75">↗</span>
                </a>
              </div>
            </div>

            {/* Bottom Footer Section */}
            <div className="flex flex-col items-center justify-between gap-4 pt-8 sm:flex-row">
              {/* Social Icons */}
              <div className="flex items-center gap-4 text-slate-400">
                <a
                  href="https://x.com"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="transition-colors hover:text-white"
                  aria-label="X"
                >
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                  </svg>
                </a>

                <a
                  href="https://github.com/cridiv/Auvis"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="transition-colors hover:text-white"
                  aria-label="GitHub"
                >
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.5 11.5 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.014 2.898-.014 3.293 0 .322.216.694.825.576C20.565 22.092 24 17.598 24 12.297c0-6.627-5.373-12-12-12" />
                  </svg>
                </a>
              </div>

              {/* Copyright Note */}
              <p className="text-sm font-normal text-slate-400">
                &copy; 2026 Auvis. Open source under Apache 2.0.
              </p>
            </div>
          </div>
        </div>
      </Blaze>
    </footer>
  );
}
