import { notFound } from "next/navigation";

// =========================================================================
// The Auvis Report - Code temporarily commented out
// =========================================================================
export default function ReportIndexPage() {
  notFound();
}

/*
import React from "react";
import Link from "next/link";
import Image from "next/image";
import type { Metadata } from "next";
import Navbar from "../components/Navbar";
import Footer from "../components/Footer";
import { getAllReports } from "@/lib/reports";
import { ArrowRight, Calendar, Clock, Sparkles, BookOpen, Tag } from "lucide-react";

export const metadata: Metadata = {
  title: "The Auvis Report - Weekly Engineering & Governance Dispatch",
  description:
    "Weekly deep dives into streaming data privacy, Kafka WAN optimization, zero-copy policy engines, and in-flight vector database security.",
};

export function OriginalReportIndexPage() {
  const reports = getAllReports();
  const latestReport = reports[0];
  const pastReports = reports.slice(1);

  return (
    <div className="flex flex-col min-h-screen bg-white">
      <Navbar />

      <main className="flex-1 pt-24 pb-20">
        // Glow Background
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 z-0 opacity-70"
          style={{
            background: `
              radial-gradient(ellipse 60% 40% at 50% 12%, rgba(123, 98, 252, 0.12) 0%, transparent 70%),
              radial-gradient(ellipse 40% 30% at 85% 20%, rgba(196, 181, 253, 0.08) 0%, transparent 60%)
            `,
          }}
        />

        <div className="relative z-10 mx-auto max-w-5xl px-6 sm:px-8">
          // Header
          <div className="mb-14 text-center sm:text-left">
            <div className="inline-flex items-center gap-2 rounded-full border border-[#7B62FC]/20 bg-[#7B62FC]/5 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-[#7B62FC]">
              <Sparkles className="h-3.5 w-3.5" />
              <span>Weekly Engineering Dispatch</span>
            </div>

            <h1
              className="mt-4 text-3xl font-bold tracking-tight text-slate-900 sm:text-5xl"
              style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
            >
              The Auvis Report
            </h1>
            <p className="mt-3.5 max-w-2xl text-base sm:text-lg text-slate-600 leading-relaxed">
              Real-world post-mortems, Go runtime telemetry, zero-copy policy compiling, and architectural lessons from building real-time data governance.
            </p>
          </div>

          // Featured / Latest Issue
          {latestReport && (
            <div className="mb-16">
              <div className="flex items-center justify-between mb-4">
                <span className="text-xs font-mono font-bold uppercase tracking-wider text-[#7B62FC] flex items-center gap-1.5">
                  <span className="h-2 w-2 rounded-full bg-[#7B62FC] animate-pulse" />
                  Latest Dispatch
                </span>
                <span className="text-xs font-medium text-slate-400">
                  Updated every Friday
                </span>
              </div>

              <div className="group relative overflow-hidden rounded-2xl border border-slate-200/90 bg-gradient-to-b from-white via-white to-slate-50/50 p-6 sm:p-9 shadow-sm transition-all hover:border-[#7B62FC]/40 hover:shadow-md">
                <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500 mb-4">
                  <span className="rounded-md bg-slate-900 px-2 py-0.5 font-mono font-semibold text-white">
                    ISSUE #{String(latestReport.issue).padStart(2, "0")}
                  </span>
                  <span className="flex items-center gap-1">
                    <Calendar className="h-3.5 w-3.5" />
                    {latestReport.date}
                  </span>
                  <span>•</span>
                  <span className="flex items-center gap-1">
                    <Clock className="h-3.5 w-3.5" />
                    {latestReport.readTime}
                  </span>
                </div>

                <Link href={`/report/${latestReport.slug}`}>
                  <h2
                    className="text-2xl sm:text-3xl font-bold tracking-tight text-slate-900 transition-colors group-hover:text-[#7B62FC]"
                    style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
                  >
                    {latestReport.title}
                  </h2>
                </Link>

                <p className="mt-3 text-slate-600 text-sm sm:text-base leading-relaxed">
                  {latestReport.excerpt}
                </p>

                // Highlights preview
                {latestReport.highlights && latestReport.highlights.length > 0 && (
                  <div className="my-5 rounded-xl border border-slate-200/80 bg-slate-50/60 p-4">
                    <span className="text-xs font-semibold uppercase tracking-wider text-slate-500 block mb-2">
                      In this issue:
                    </span>
                    <ul className="space-y-1.5 text-xs sm:text-sm text-slate-700">
                      {latestReport.highlights.map((h, i) => (
                        <li key={i} className="flex items-start gap-2">
                          <span className="text-[#7B62FC] font-bold">›</span>
                          <span>{h}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="flex flex-wrap items-center justify-between gap-4 pt-2">
                  <div className="flex flex-wrap items-center gap-1.5">
                    {latestReport.tags.map((t) => (
                      <span
                        key={t}
                        className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600 border border-slate-200/60"
                      >
                        #{t}
                      </span>
                    ))}
                  </div>

                  <Link
                    href={`/report/${latestReport.slug}`}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-[#7B62FC] active:scale-[0.98]"
                  >
                    <span>Read Issue #{latestReport.issue}</span>
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                </div>
              </div>
            </div>
          )}

          // Past Issues Section
          <div className="border-t border-slate-200/80 pt-12">
            <div className="flex items-center justify-between mb-8">
              <div>
                <h3
                  className="text-2xl font-bold tracking-tight text-slate-900"
                  style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
                >
                  Archive &amp; Previous Dispatches
                </h3>
                <p className="text-sm text-slate-500 mt-1">
                  Explore all engineering reports published by the Auvis team.
                </p>
              </div>
              <div className="text-xs font-mono text-slate-400">
                {reports.length} {reports.length === 1 ? "Issue" : "Issues"} Total
              </div>
            </div>

            {pastReports.length > 0 ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {pastReports.map((report) => (
                  <div
                    key={report.slug}
                    className="group flex flex-col justify-between rounded-xl border border-slate-200 bg-white p-6 shadow-xs transition-all hover:border-[#7B62FC]/50 hover:shadow-sm"
                  >
                    <div>
                      <div className="flex items-center justify-between text-xs text-slate-500 mb-3">
                        <span className="font-mono font-semibold text-slate-800 bg-slate-100 border border-slate-200 px-2 py-0.5 rounded">
                          ISSUE #{String(report.issue).padStart(2, "0")}
                        </span>
                        <span>{report.date}</span>
                      </div>

                      <Link href={`/report/${report.slug}`}>
                        <h4
                          className="text-lg font-bold tracking-tight text-slate-900 group-hover:text-[#7B62FC] transition-colors"
                          style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
                        >
                          {report.title}
                        </h4>
                      </Link>

                      <p className="mt-2 text-xs sm:text-sm text-slate-600 leading-relaxed line-clamp-3">
                        {report.excerpt}
                      </p>
                    </div>

                    <div className="mt-5 flex items-center justify-between border-t border-slate-100 pt-3">
                      <div className="flex items-center gap-1 text-xs text-slate-400">
                        <Clock className="h-3 w-3" />
                        <span>{report.readTime}</span>
                      </div>
                      <Link
                        href={`/report/${report.slug}`}
                        className="text-xs font-semibold text-[#7B62FC] flex items-center gap-1 group-hover:translate-x-0.5 transition-transform"
                      >
                        <span>Read</span>
                        <ArrowRight className="h-3 w-3" />
                      </Link>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-slate-200 p-8 text-center text-sm text-slate-500">
                Issue #02 is landing next week! Check back every Friday.
              </div>
            )}
          </div>

          // Newsletter / RSS Callout Card
          <div className="mt-16 rounded-2xl border border-slate-800 bg-[#0B0C11] p-8 text-white">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6">
              <div>
                <span className="inline-block rounded-md bg-[#7B62FC]/20 px-2 py-0.5 font-mono text-xs font-semibold text-[#C4B5FD] mb-2">
                  STAY IN THE LOOP
                </span>
                <h3
                  className="text-xl sm:text-2xl font-bold tracking-tight text-white"
                  style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
                >
                  Follow the weekly build in public
                </h3>
                <p className="text-sm text-slate-300 mt-1 max-w-xl">
                  We share raw numbers, failed experiments, Go profiling captures, and real compliance traps every week.
                </p>
              </div>

              <div className="flex items-center gap-3">
                <a
                  href="https://github.com/cridiv/Auvis"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 rounded-lg bg-white px-4 py-2.5 text-xs sm:text-sm font-semibold text-slate-900 transition-all hover:bg-slate-100 active:scale-[0.98]"
                >
                  <Image src="/icon/github.svg" alt="" width={16} height={16} className="h-4 w-4" />
                  <span>Star on GitHub</span>
                </a>
              </div>
            </div>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  );
}
*/
