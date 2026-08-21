"use client";

import React, { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  ShieldCheck,
  Scan,
  KeyRound,
  Database,
  FileCheck2,
  Check,
  ArrowRight,
  Zap,
  Lock,
  Sparkles,
  Layers,
  Terminal,
  Activity,
} from "lucide-react";

interface Capability {
  number: string;
  titlePrefix: string;
  titleSuffix: string;
  description: string;
  bullets: string[];
  ctaText?: string;
  ctaLink?: string;
  art: React.ReactNode;
}

const CAPABILITIES: Capability[] = [
  {
    number: "01",
    titlePrefix: "Policy Enforcement, ",
    titleSuffix: "rule by rule in flight",
    description:
      "Mask, redact, tokenize, or drop any field in real time. Dynamic conditions evaluate row-level rules with deterministic priority resolution.",
    bullets: [
      "Mask, redact, tokenize, or drop any field",
      "Row-level filtering with dynamic conditions",
      "Priority-based conflict resolution (deterministic & auditable)",
      "Hot-reload: policy changes take effect in seconds, no restart",
    ],
    art: (
      <div className="relative w-full max-w-sm">
        {/* Background Layer Card */}
        <div className="absolute -top-3 left-4 right-0 h-full rounded-[10px] border border-slate-200 bg-slate-100/70">
          <p className="px-3 pt-2 font-mono text-[9px] uppercase tracking-[0.12em] text-slate-400">
            Pipeline Policies
          </p>
        </div>

        {/* Foreground Window */}
        <div className="relative overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/60">
          <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/80 px-3 py-2">
            <div className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-rose-400" />
              <span className="h-2 w-2 rounded-full bg-amber-400" />
              <span className="h-2 w-2 rounded-full bg-emerald-400" />
              <span className="ml-1.5 font-mono text-[9.5px] text-slate-500">
                mask-pii.yaml
              </span>
            </div>
            <span className="rounded bg-purple-50 px-1.5 py-0.5 font-mono text-[8.5px] font-semibold text-[#7B62FC]">
              Hot-reload
            </span>
          </div>

          <ul className="divide-y divide-slate-100 px-3 py-1 font-mono text-[10px]">
            <li className="flex items-center justify-between py-2">
              <span className="text-slate-500">
                email &rarr;{" "}
                <span className="text-purple-600 font-semibold">mask</span>
              </span>
              <span className="text-slate-400 text-[9px]">***@example.com</span>
            </li>
            <li className="flex items-center justify-between py-2">
              <span className="text-slate-500">
                salary &rarr;{" "}
                <span className="text-rose-500 font-semibold">remove</span>
              </span>
              <span className="text-rose-400 text-[9px]">
                role != &apos;hr&apos;
              </span>
            </li>
            <li className="flex items-center justify-between py-2">
              <span className="text-slate-500">
                phone &rarr;{" "}
                <span className="text-purple-600 font-semibold">mask</span>
              </span>
              <span className="text-slate-400 text-[9px]">***-***-0198</span>
            </li>
            <li className="flex items-center justify-between py-2">
              <span className="text-slate-500">
                ssn &rarr;{" "}
                <span className="text-rose-500 font-semibold">drop</span>
              </span>
              <span className="text-emerald-600 text-[9px]">GDPR enforced</span>
            </li>
          </ul>

          <div className="flex items-center justify-between border-t border-slate-100 bg-slate-50/50 px-3 py-1.5 text-[9px] text-slate-400">
            <span className="flex items-center gap-1 text-emerald-600 font-medium">
              <Zap className="h-2.5 w-2.5" /> Eval &lt;5ms
            </span>
            <span>Explicit &gt; Wildcard</span>
          </div>
        </div>
      </div>
    ),
  },
  {
    number: "02",
    titlePrefix: "PII Detection: ",
    titleSuffix: "catches what regex alone misses",
    description:
      "Mathematical check-digit validation combined with sliding window semantic context boosting and LLM fallback for unstructured names and addresses.",
    bullets: [
      "Regex + check-digit validation (Luhn, IBAN, ABA routing)",
      "LLM fallback (Claude) for names, addresses, freeform text",
      "Confidence scoring (0.0–1.0) with configurable thresholds",
      "Every match logged, not just the ones acted on",
    ],
    art: (
      <div className="w-full max-w-sm overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/60">
        <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/80 px-3 py-2">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="ml-1.5 font-mono text-[9.5px] text-slate-500">
              Hybrid PII Scanner
            </span>
          </div>
          <span className="rounded bg-indigo-50 px-1.5 py-0.5 font-mono text-[8.5px] text-indigo-600">
            Confidence &gt; 0.70
          </span>
        </div>

        <div className="p-3.5 space-y-2.5">
          <div className="rounded-[6px] border border-slate-100 bg-slate-50/70 p-2 text-[10px]">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">
                1. Math Check-Digit
              </span>
              <span className="text-emerald-600 text-[9px] font-mono font-medium">
                Luhn 1.00 ✓
              </span>
            </div>
            <p className="mt-0.5 text-slate-500 font-mono text-[9px]">
              4111-****-****-4242
            </p>
          </div>

          <div className="rounded-[6px] border border-slate-100 bg-slate-50/70 p-2 text-[10px]">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">
                2. Context Window Boost
              </span>
              <span className="text-purple-600 text-[9px] font-mono font-medium">
                +40 chars
              </span>
            </div>
            <p className="mt-0.5 text-slate-500 text-[9px]">
              Semantic anchor &apos;SSN&apos; verified
            </p>
          </div>

          <div className="rounded-[6px] border border-slate-100 bg-slate-50/70 p-2 text-[10px]">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">
                3. LLM Fallback (Claude)
              </span>
              <span className="text-amber-600 text-[9px] font-mono font-medium">
                200ms guard
              </span>
            </div>
            <p className="mt-0.5 text-slate-500 text-[9px]">
              Named entity: &quot;Margaret Okonkwo&quot;
            </p>
          </div>
        </div>

        <div className="border-t border-slate-100 px-3 py-1.5 text-[9px] text-slate-400 flex justify-between">
          <span>Span Resolution: Overlaps merged</span>
          <span className="text-[#7B62FC] font-medium">Highest score wins</span>
        </div>
      </div>
    ),
  },
  {
    number: "03",
    titlePrefix: "Access Control: ",
    titleSuffix: "permissions travel with the data",
    description:
      "Extract JWT claims, query IdPs (Okta, Auth0), and propagate source access controls directly into vector embeddings with automatic conflict resolution.",
    bullets: [
      "JWT claim extraction from your existing auth",
      "Optional IdP lookup (Okta, Auth0, LDAP)",
      "Wildcard + explicit-role precedence",
      "Specificity resolves conflicts automatically without priority tuning",
    ],
    art: (
      <div className="w-full max-w-sm overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/60">
        <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/80 px-3 py-2">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="ml-1.5 font-mono text-[9.5px] text-slate-500">
              Identity &amp; Context
            </span>
          </div>
          <span className="rounded bg-purple-50 px-1.5 py-0.5 font-mono text-[8.5px] text-[#7B62FC]">
            Fail-closed
          </span>
        </div>

        <div className="p-3.5 space-y-2 font-mono text-[10px]">
          <div className="rounded-[6px] border border-slate-100 bg-slate-50/80 p-2.5 space-y-1">
            <div className="text-slate-400 text-[8.5px] uppercase tracking-wider">
              Resolved JWT Context
            </div>
            <div className="text-slate-700">
              user.role:{" "}
              <span className="text-purple-600 font-semibold">
                &quot;analyst&quot;
              </span>
            </div>
            <div className="text-slate-700">
              user.region:{" "}
              <span className="text-slate-900">&quot;eu-west-1&quot;</span>
            </div>
            <div className="text-slate-700">
              user.clearance:{" "}
              <span className="text-slate-900">&quot;L2-restricted&quot;</span>
            </div>
          </div>

          <div className="rounded-[6px] border border-purple-100 bg-purple-50/40 p-2 flex items-center justify-between">
            <span className="text-slate-600">Rule Specificity Engine:</span>
            <span className="text-[#7B62FC] font-semibold text-[9.5px]">
              Explicit &gt; Wildcard (*)
            </span>
          </div>

          <p className="text-[9px] text-slate-400 pt-1 font-sans">
            Source permissions carried through to the vector index on write.
          </p>
        </div>
      </div>
    ),
  },
  {
    number: "04",
    titlePrefix: "Destination Sinks: ",
    titleSuffix: "fail-closed, not fail-open",
    description:
      "Native sync writes to Postgres, pgvector, Weaviate, and Pinecone guarded by 3-state circuit breakers. Secondary sink outages never impact client response times.",
    bullets: [
      "Native support for Postgres, pgvector, Weaviate, Pinecone",
      "Independent circuit breakers per destination",
      "Client always gets a governed response. Sink outages never add latency",
      "Auto-schema provisioning on first write",
    ],
    ctaText: "Explore Sinks",
    ctaLink: "#docs",
    art: (
      <div className="w-full max-w-sm overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/60">
        <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/80 px-3 py-2">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="ml-1.5 font-mono text-[9.5px] text-slate-500">
              Sink Mesh Status
            </span>
          </div>
          <span className="rounded bg-emerald-50 px-1.5 py-0.5 font-mono text-[8.5px] text-emerald-600">
            All Systems Normal
          </span>
        </div>

        <div className="grid grid-cols-2 gap-2 p-3 text-[10px] font-mono">
          <div className="rounded-[6px] border border-slate-100 bg-slate-50/80 p-2">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">pgvector</span>
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            </div>
            <p className="mt-1 text-[8.5px] text-slate-400">
              table &lt;-&gt; vector(3)
            </p>
          </div>

          <div className="rounded-[6px] border border-slate-100 bg-slate-50/80 p-2">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">Weaviate</span>
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            </div>
            <p className="mt-1 text-[8.5px] text-slate-400">
              UUIDv5 safe hashing
            </p>
          </div>

          <div className="rounded-[6px] border border-slate-100 bg-slate-50/80 p-2">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">Pinecone</span>
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            </div>
            <p className="mt-1 text-[8.5px] text-slate-400">Flat-mapped meta</p>
          </div>

          <div className="rounded-[6px] border border-slate-100 bg-slate-50/80 p-2">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">Postgres</span>
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            </div>
            <p className="mt-1 text-[8.5px] text-slate-400">Fail-closed sync</p>
          </div>
        </div>

        <div className="border-t border-slate-100 px-3 py-1.5 text-[9px] text-slate-400 flex justify-between">
          <span>Circuit Breaker Guard</span>
          <span className="text-emerald-600 font-medium">
            &lt;0.1ms fast-fail bypass
          </span>
        </div>
      </div>
    ),
  },
  {
    number: "05",
    titlePrefix: "Audit & Compliance: ",
    titleSuffix: "proof, not promises",
    description:
      "Structured four-bucket field disposition logging in JSONL. Ingestible directly by Splunk, Datadog, and ClickHouse with zero PII exposure.",
    bullets: [
      "Four-bucket field disposition per request (masked / removed / allowed / denied)",
      "JSONL output (native for Splunk, Datadog, ClickHouse)",
      "Configurable fsync durability (speed vs. zero data loss)",
      "Built for GDPR/HIPAA-style reporting from day one",
    ],
    ctaText: "View Documentation",
    ctaLink: "#docs",
    art: (
      <div className="w-full max-w-sm overflow-hidden rounded-[10px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/60">
        <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50/80 px-3 py-2">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="h-2 w-2 rounded-full bg-slate-300" />
            <span className="ml-1.5 font-mono text-[9.5px] text-slate-500">
              Audit Trail (JSONL)
            </span>
          </div>
          <span className="rounded bg-purple-50 px-1.5 py-0.5 font-mono text-[8.5px] text-[#7B62FC]">
            Immutable
          </span>
        </div>

        <ul className="divide-y divide-slate-100 px-3 py-1 font-mono text-[9.5px]">
          <li className="flex items-center gap-2 py-2">
            <span className="h-1.5 w-1.5 rounded-full bg-purple-500" />
            <span className="text-slate-400">1042</span>
            <span className="flex-1 text-slate-800 truncate font-semibold">
              fields_masked: [&quot;email&quot;, &quot;phone&quot;]
            </span>
            <span className="text-slate-400 text-[8.5px]">GDPR</span>
          </li>
          <li className="flex items-center gap-2 py-2">
            <span className="h-1.5 w-1.5 rounded-full bg-rose-500" />
            <span className="text-slate-400">1041</span>
            <span className="flex-1 text-slate-800 truncate font-semibold">
              fields_removed: [&quot;ssn&quot;, &quot;salary&quot;]
            </span>
            <span className="text-slate-400 text-[8.5px]">HIPAA</span>
          </li>
          <li className="flex items-center gap-2 py-2">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            <span className="text-slate-400">1040</span>
            <span className="flex-1 text-slate-800 truncate font-semibold">
              fields_allowed: [&quot;name&quot;, &quot;id&quot;]
            </span>
            <span className="text-slate-400 text-[8.5px]">Pass</span>
          </li>
        </ul>

        <div className="border-t border-slate-100 px-3 py-1.5 text-[9px] text-slate-400 flex justify-between">
          <span>Durability: Speed vs. Always</span>
          <span className="text-slate-700 font-mono">19.7k req/s</span>
        </div>
      </div>
    ),
  },
];

const Features = () => {
  const [activeIdx, setActiveIdx] = useState(0);
  const [isPaused, setIsPaused] = useState(false);
  const timerRef = useRef<NodeJS.Timeout | null>(null);

  // Auto-advance tabs every 7 seconds
  useEffect(() => {
    if (isPaused) return;

    timerRef.current = setInterval(() => {
      setActiveIdx((prev) => (prev + 1) % CAPABILITIES.length);
    }, 7000);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [isPaused]);

  const activeCap = CAPABILITIES[activeIdx];

  return (
    <section
      id="features"
      className="scroll-mt-28 bg-slate-50/50 py-24 sm:py-36"
    >
      <div className="mx-auto w-full max-w-[1440px] px-6">
        {/* Showcase Grid Box */}
        <div
          onMouseEnter={() => setIsPaused(true)}
          onMouseLeave={() => setIsPaused(false)}
          className="cap-showcase mx-auto grid max-w-[1080px] overflow-hidden rounded-[12px] border border-slate-200/90 bg-white shadow-xl shadow-slate-200/50 lg:min-h-[34rem] lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]"
        >
          {/* Left Column: Headline, Active Capability Text, and Tab Tracks */}
          <div className="flex flex-col justify-between p-6 sm:p-10">
            <div>
              {/* Section Main Header */}
              <h2 className="max-w-md text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl">
                <span className="block">One software.</span>
                <span className="block text-slate-400">
                  Every pipeline governed on the record.
                </span>
              </h2>
              <p className="mt-3 max-w-sm text-sm text-slate-500">
                Everything here ships from the first seat. Nothing is an
                upgrade.
              </p>
            </div>

            {/* Active Capability Panel Text */}
            <div className="mt-8 min-h-[160px]">
              <AnimatePresence mode="wait">
                <motion.div
                  key={activeIdx}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -8 }}
                  transition={{ duration: 0.25, ease: "easeInOut" }}
                  className="cap-panel"
                >
                  <p className="font-mono text-xs font-semibold text-[#7B62FC]">
                    {activeCap.number} / 05
                  </p>
                  <h3 className="mt-3 max-w-sm text-xl font-bold tracking-tight text-slate-900">
                    {activeCap.titlePrefix}
                    <span className="text-slate-400 font-normal">
                      {activeCap.titleSuffix}
                    </span>
                  </h3>
                  <p className="mt-3 max-w-sm text-sm leading-relaxed text-slate-600">
                    {activeCap.description}
                  </p>

                  {/* Bullet points summary */}
                  <ul className="mt-4 space-y-1.5">
                    {activeCap.bullets.slice(0, 2).map((bullet, idx) => (
                      <li
                        key={idx}
                        className="flex items-center gap-2 text-xs text-slate-500"
                      >
                        <Check className="h-3 w-3 text-[#7B62FC]" />
                        <span>{bullet}</span>
                      </li>
                    ))}
                  </ul>

                  {activeCap.ctaText && (
                    <div className="mt-6">
                      <a
                        href={activeCap.ctaLink || "#"}
                        className="group inline-flex items-center gap-2 rounded-md bg-slate-900 px-4 py-2 text-xs font-medium text-white transition-all hover:bg-[#7B62FC] active:scale-[0.98]"
                      >
                        <span>{activeCap.ctaText}</span>
                        <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                      </a>
                    </div>
                  )}
                </motion.div>
              </AnimatePresence>
            </div>

            {/* Segmented Tab Tracks at Bottom */}
            <div
              role="tablist"
              aria-label="Capabilities"
              className="cap-tabs mt-8 flex gap-2 border-t border-slate-100 pt-6"
            >
              {CAPABILITIES.map((cap, idx) => {
                const isActive = activeIdx === idx;
                return (
                  <button
                    key={idx}
                    type="button"
                    role="tab"
                    aria-selected={isActive}
                    onClick={() => setActiveIdx(idx)}
                    className="relative flex-1 py-2 cursor-pointer group text-left focus:outline-none"
                  >
                    {/* Track Background */}
                    <div className="h-1 w-full overflow-hidden rounded-full bg-slate-100 transition-colors group-hover:bg-slate-200">
                      {isActive && (
                        <motion.div
                          key={`progress-${idx}`}
                          initial={{ width: "0%" }}
                          animate={{ width: isPaused ? "100%" : "100%" }}
                          transition={{
                            duration: isPaused ? 0 : 7,
                            ease: "linear",
                          }}
                          className="h-full bg-[#7B62FC]"
                        />
                      )}
                    </div>
                    <span className="mt-1.5 block font-mono text-[9px] text-slate-400 group-hover:text-slate-700">
                      {cap.number}
                    </span>
                    <span className="sr-only">
                      {cap.titlePrefix} {cap.titleSuffix}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Right Column: Interactive Visual Plate / Artwork */}
          <div className="flex min-h-[18rem] items-center justify-center overflow-hidden border-t border-slate-100 bg-slate-50/60 p-6 sm:min-h-[24rem] sm:p-10 lg:border-t-0 lg:border-l">
            <div className="w-full flex items-center justify-center">
              <AnimatePresence mode="wait">
                <motion.div
                  key={activeIdx}
                  initial={{ opacity: 0, scale: 0.96 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.96 }}
                  transition={{ duration: 0.3, ease: "easeInOut" }}
                  className="w-full flex items-center justify-center"
                >
                  {activeCap.art}
                </motion.div>
              </AnimatePresence>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Features;
