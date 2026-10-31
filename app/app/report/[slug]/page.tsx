import { notFound } from "next/navigation";

// =========================================================================
// The Auvis Report Detail - Code temporarily commented out
// =========================================================================

export async function generateStaticParams() {
  return [];
}

export default function ReportPostPage() {
  notFound();
}

/*
import React from "react";
import Link from "next/link";
import Image from "next/image";
import type { Metadata } from "next";
import Navbar from "../../components/Navbar";
import Footer from "../../components/Footer";
import { getAllReports, getReportBySlug } from "@/lib/reports";
import { compileMDX } from "next-mdx-remote/rsc";
import { mdxComponents } from "../components/mdx-components";
import { ArrowLeft, ArrowRight, Calendar, Clock, MessageSquare, Share2, Sparkles } from "lucide-react";
import remarkGfm from "remark-gfm";

interface PageProps {
  params: Promise<{ slug: string }>;
}

export async function OriginalReportPostPage({ params }: PageProps) {
  const { slug } = await params;
  const reportData = getReportBySlug(slug);

  if (!reportData) {
    notFound();
  }

  const { meta, rawContent } = reportData;
  const allReports = getAllReports();
  const currentIndex = allReports.findIndex((r) => r.slug === slug);
  const prevReport = currentIndex < allReports.length - 1 ? allReports[currentIndex + 1] : null;
  const nextReport = currentIndex > 0 ? allReports[currentIndex - 1] : null;

  const plugin = (remarkGfm as unknown as { default?: unknown }).default || remarkGfm;
  const { content } = await compileMDX({
    source: rawContent,
    components: mdxComponents,
    options: {
      mdxOptions: {
        remarkPlugins: [plugin as any],
      },
    },
  });

  return (
    <div className="flex flex-col min-h-screen bg-white">
      <Navbar />

      <main className="flex-1 pt-24 pb-20">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 z-0 opacity-60"
          style={{
            background: `
              radial-gradient(ellipse 50% 30% at 50% 10%, rgba(123, 98, 252, 0.08) 0%, transparent 60%)
            `,
          }}
        />

        <article className="relative z-10 mx-auto max-w-3xl px-6 sm:px-8">
          <div className="mb-8">
            <Link
              href="/report"
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 hover:text-[#7B62FC] transition-colors"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>Back to all dispatches</span>
            </Link>
          </div>

          <header className="mb-10 pb-8 border-b border-slate-200/80">
            <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500 mb-4">
              <span className="rounded-md bg-slate-900 px-2.5 py-1 font-mono font-bold text-white text-xs">
                ISSUE #{String(meta.issue).padStart(2, "0")}
              </span>
              <span className="flex items-center gap-1">
                <Calendar className="h-3.5 w-3.5" />
                {meta.date}
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <Clock className="h-3.5 w-3.5" />
                {meta.readTime}
              </span>
            </div>

            <h1
              className="text-3xl sm:text-4xl lg:text-5xl font-bold tracking-tight text-slate-900 leading-tight"
              style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
            >
              {meta.title}
            </h1>

            <p className="mt-4 text-lg text-slate-600 leading-relaxed">
              {meta.excerpt}
            </p>

            <div className="mt-6 flex items-center justify-between gap-4 pt-4 border-t border-slate-100">
              <div className="flex items-center gap-3">
                <div className="h-9 w-9 rounded-full bg-slate-900 flex items-center justify-center text-white font-mono text-xs font-bold">
                  {meta.author.name.split(" ").map((n: string) => n[0]).join("")}
                </div>
                <div>
                  <div className="text-xs font-semibold text-slate-900">{meta.author.name}</div>
                  <div className="text-[11px] text-slate-500">{meta.author.role}</div>
                </div>
              </div>

              <div className="flex items-center gap-1.5">
                {meta.tags.map((t: string) => (
                  <span
                    key={t}
                    className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600"
                  >
                    #{t}
                  </span>
                ))}
              </div>
            </div>
          </header>

          <div className="prose prose-slate max-w-none">
            {content}
          </div>

          <div className="mt-16 pt-8 border-t border-slate-200">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {prevReport ? (
                <Link
                  href={`/report/${prevReport.slug}`}
                  className="group flex flex-col p-4 rounded-xl border border-slate-200 bg-white hover:border-[#7B62FC]/50 transition-all text-left"
                >
                  <span className="text-[11px] font-mono text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-1">
                    <ArrowLeft className="h-3 w-3 group-hover:-translate-x-0.5 transition-transform" />
                    Previous Dispatch
                  </span>
                  <span className="text-sm font-semibold text-slate-900 group-hover:text-[#7B62FC] transition-colors line-clamp-1">
                    #{prevReport.issue} • {prevReport.title}
                  </span>
                </Link>
              ) : (
                <div className="hidden sm:block" />
              )}

              {nextReport ? (
                <Link
                  href={`/report/${nextReport.slug}`}
                  className="group flex flex-col p-4 rounded-xl border border-slate-200 bg-white hover:border-[#7B62FC]/50 transition-all text-right items-end sm:col-start-2"
                >
                  <span className="text-[11px] font-mono text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-1">
                    Next Dispatch
                    <ArrowRight className="h-3 w-3 group-hover:translate-x-0.5 transition-transform" />
                  </span>
                  <span className="text-sm font-semibold text-slate-900 group-hover:text-[#7B62FC] transition-colors line-clamp-1">
                    #{nextReport.issue} • {nextReport.title}
                  </span>
                </Link>
              ) : (
                <div className="hidden sm:block" />
              )}
            </div>
          </div>
        </article>
      </main>

      <Footer />
    </div>
  );
}
*/
