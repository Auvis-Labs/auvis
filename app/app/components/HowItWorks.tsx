"use client";

import React from "react";
import { motion } from "framer-motion";

interface Step {
  number: string;
  shortTitle: string;
  headline: string;
  description: string;
}

const STEPS: Step[] = [
  {
    number: "01",
    shortTitle: "Intercept",
    headline: "Every record, before it lands anywhere.",
    description:
      "Auvis sits inside your ingestion pipelines; Kafka streams, Confluence syncs, database CDC intercepting records in flight before they reach vector databases or LLM context windows.",
  },
  {
    number: "02",
    shortTitle: "Evaluate",
    headline: "Matched against policy in under 5ms.",
    description:
      "Each record is evaluated against your defined rules; role, resource, and field using a compiled decision tree with zero added latency your team will notice.",
  },
  {
    number: "03",
    shortTitle: "Transform & Write",
    headline: "Mask, redact, and fail-closed sync.",
    description:
      "Sensitive fields are rewritten based on policy rules, and governed records are written synchronously to Postgres, pgvector, Weaviate, or Pinecone with fast-fail circuit breakers.",
  },
  {
    number: "04",
    shortTitle: "Audit",
    headline: "Proof for every decision, automatically.",
    description:
      "What was masked, removed, allowed, and denied is logged asynchronously as JSONL ready for Splunk, Datadog, and compliance audits without touching the hot path.",
  },
];

const HowItWorks = () => {
  return (
    <section
      id="how-it-works"
      className="scroll-mt-28 border-t border-slate-200/80 bg-white py-24 sm:py-36"
    >
      <div className="mx-auto w-full max-w-[1440px] px-6">
        {/* Section Header */}
        <div className="max-w-4xl">
          <p className="font-mono text-xs uppercase tracking-[0.14em] text-slate-400">
            THE PIPELINE ARCHITECTURE
          </p>
          <h2 className="mt-4 text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl lg:text-5xl">
            <span className="block">
              Set the standard, connect your agents, keep the record.
            </span>
            <span className="block text-slate-400 font-normal">
              {" "}
              One platform.
            </span>
          </h2>
          <p className="mt-4 max-w-2xl text-base text-slate-600">
            Auvis operates as a transparent proxy and streaming engine inside
            your network, enforcing governance policies in four continuous,
            deterministic steps.
          </p>
        </div>

        {/* 4 Square Steps Grid */}
        <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {STEPS.map((step, idx) => (
            <motion.article
              key={idx}
              whileHover={{ y: -6, scale: 1.01 }}
              transition={{ duration: 0.2 }}
              className="group flex aspect-square flex-col justify-between rounded-2xl border border-slate-200/90 bg-white p-6 sm:p-7 shadow-sm transition-all hover:border-[#7B62FC]/50 hover:shadow-xl hover:shadow-purple-100/50"
            >
              {/* Top Row: Step Number & Badge */}
              <div className="flex items-center justify-between">
                <span className="font-mono text-sm font-bold text-[#7B62FC]">
                  {step.number}
                </span>
                {/* <span className="rounded-full bg-slate-100 px-2.5 py-0.5 font-mono text-[10px] font-medium text-slate-600 transition-colors group-hover:bg-purple-50 group-hover:text-[#7B62FC]">
                  {step.shortTitle}
                </span> */}
              </div>

              {/* Middle / Main Content */}
              <div className="my-auto">
                <h3 className="text-lg font-bold tracking-tight text-slate-900 transition-colors group-hover:text-[#7B62FC]">
                  {step.shortTitle}
                </h3>
                <p className="mt-1.5 text-xs font-semibold text-slate-700 leading-snug">
                  {step.headline}
                </p>
                <p className="mt-3 text-xs leading-relaxed text-slate-500 line-clamp-5 sm:line-clamp-6">
                  {step.description}
                </p>
              </div>

              {/* Bottom Subtle Indicator */}
              <div className="flex items-center gap-1.5 pt-2">
                <span className="h-1.5 w-1.5 rounded-full bg-[#7B62FC]/40 group-hover:bg-[#7B62FC] transition-colors" />
                <span className="font-mono text-[9.5px] text-slate-400 group-hover:text-slate-600 transition-colors">
                  0{idx + 1} / 04
                </span>
              </div>
            </motion.article>
          ))}
        </div>
      </div>
    </section>
  );
};

export default HowItWorks;
