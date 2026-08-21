"use client";

import React, { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChevronDown, HelpCircle } from "lucide-react";

interface FAQItem {
  question: string;
  answer: string;
}

const faqs: FAQItem[] = [
  {
    question: "Does Auvis add latency to my pipeline?",
    answer:
      "Policy evaluation runs in under 5ms per record using a compiled decision tree, and the gateway's internal processing overhead is sub-millisecond (+0.70ms). In 400,000-request load tests across dedicated VPC instances, Auvis delivers 19.7k req/s (19,682 req/sec) with 0% error rate. Secondary destination sink writes are also decoupled with <0.1ms circuit breaker fast-fails, so slow or offline vector databases never delay client HTTP responses.",
  },
  {
    question: "What happens if a destination database goes down?",
    answer:
      "Auvis fails closed on the write, not on the response. Independent circuit breakers guard each destination (Postgres, pgvector, Weaviate, Pinecone). If one trips, the write is fast-failed and logged with the exact reason, but your client still gets their governed data back. Nothing silently drops, and nothing ungoverned slips through either.",
  },
  {
    question: "Do I need to change my application code?",
    answer:
      "No. Auvis sits as a reverse proxy inside your ingestion pipeline. You point it at your upstream and your destinations, write policy in YAML, and it enforces from there. No SDK to integrate, no application-level changes.",
  },
  {
    question: "How accurate is the PII detection? Will it over-mask?",
    answer:
      "Detection runs in hybrid mode by default: regex with mathematical check-digit validation (Luhn, IBAN, ABA routing) for structured data like card numbers and SSNs, with an LLM fallback for unstructured text like names and addresses. Every match gets a confidence score, and you set the threshold, so you control the precision/recall tradeoff instead of accepting a black box.",
  },
  {
    question: "Is this open source?",
    answer:
      "Yes. The core gateway, policy engine, transformation pipeline, PII scanner, and audit logging are Apache 2.0 and stay free forever. Hosted or enterprise features (managed control plane, SSO, automated compliance reports) are planned as optional paid add-ons later, but the governance engine itself is never paywalled.",
  },
  {
    question: "How is this different from an LLM gateway or a data catalog?",
    answer:
      "LLM gateways filter prompts at inference time, and by then your data's already sitting in the vector index. Data catalogs document lineage but don't enforce anything at runtime. Auvis sits earlier, at the pipeline layer, so nothing ungoverned ever reaches the vector database in the first place.",
  },
  {
    question: "Can I change policies without downtime?",
    answer:
      "Yes. Policy changes hot-reload and take effect within seconds, with no restart and no dropped requests.",
  },
  {
    question: "What compliance frameworks does this help with?",
    answer:
      "The four-bucket audit trail (masked, removed, allowed, denied) plus JSONL output maps directly to GDPR Article 30 records, HIPAA access logging, and PCI-DSS cardholder data controls, feeding straight into Splunk, Datadog, or ClickHouse for reporting.",
  },
];

const Faqs = () => {
  const [openIndex, setOpenIndex] = useState<number | null>(0);

  const toggleAccordion = (index: number) => {
    setOpenIndex(openIndex === index ? null : index);
  };

  return (
    <section
      id="faqs"
      className="relative w-full border-t border-slate-200/80 bg-white py-28 px-6 sm:px-8 sm:py-36"
    >
      {/* Background Ambient Glow */}
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-purple-50/50 via-slate-50/20 to-white" />

      <div className="mx-auto flex max-w-4xl flex-col gap-14 sm:gap-16">
        {/* Header */}
        <div className="w-full space-y-3 text-center">
          {/* <div className="inline-flex items-center gap-2 rounded-full border border-purple-200 bg-purple-50/80 px-3.5 py-1 text-xs font-semibold text-[#7B62FC]">
            <HelpCircle className="h-3.5 w-3.5" />
            <span>FREQUENTLY ASKED QUESTIONS</span>
          </div> */}
          <h2 className="text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl lg:text-5xl">
            Frequently Asked Questions
          </h2>
          <p className="mx-auto max-w-xl text-sm text-slate-600 sm:text-base">
            Everything you need to know about pipeline governance, latency
            overhead, and destination fail-closed security.
          </p>
        </div>

        {/* Accordion List */}
        <div className="flex w-full flex-col">
          {faqs.map((faq, index) => {
            const isOpen = openIndex === index;

            return (
              <div
                key={index}
                className="border-b border-slate-200/80 last:border-b-0 transition-colors"
              >
                <button
                  type="button"
                  onClick={() => toggleAccordion(index)}
                  className="group flex w-full items-center justify-between py-6 text-left outline-none cursor-pointer"
                >
                  <h3
                    className={`text-lg sm:text-xl font-semibold tracking-tight pr-4 transition-colors ${isOpen
                        ? "text-[#7B62FC]"
                        : "text-slate-900 group-hover:text-[#7B62FC]"
                      }`}
                  >
                    {faq.question}
                  </h3>
                  <motion.div
                    animate={{ rotate: isOpen ? 180 : 0 }}
                    transition={{ duration: 0.2 }}
                    className="shrink-0"
                  >
                    <ChevronDown
                      className={`h-5 w-5 transition-colors ${isOpen
                          ? "text-[#7B62FC]"
                          : "text-slate-400 group-hover:text-slate-700"
                        }`}
                    />
                  </motion.div>
                </button>
                <AnimatePresence initial={false}>
                  {isOpen && (
                    <motion.div
                      initial={{ height: 0, opacity: 0 }}
                      animate={{ height: "auto", opacity: 1 }}
                      exit={{ height: 0, opacity: 0 }}
                      transition={{ duration: 0.28, ease: "easeInOut" }}
                      className="overflow-hidden"
                    >
                      <p className="max-w-3xl pb-8 text-sm leading-relaxed text-slate-600 sm:text-base">
                        {faq.answer}
                      </p>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
};

export default Faqs;
